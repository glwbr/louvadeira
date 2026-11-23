package notify_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"louvadeira/internal/store"
)

func remindersIn(messages []message) []message {
	var out []message
	for _, m := range messages {
		if strings.HasPrefix(m.text, "Paz, pessoal! 🎶 Escala do *Louvor* de ") && strings.Contains(m.text, "\n*Culto de") {
			out = append(out, m)
		}
	}
	return out
}

func (w *world) publish(month string) {
	w.t.Helper()
	ctx := context.Background()
	if err := w.store.GenerateDraft(ctx, w.ministry.ID, month, w.clock); err != nil {
		w.t.Fatal(err)
	}
	if err := w.store.PublishSchedule(ctx, w.ministry.ID, month, w.clock); err != nil {
		w.t.Fatal(err)
	}
}

func TestEachServiceIsRemindedInTheGroupTwoDaysBeforeAt18h(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.clock = time.Date(2026, 11, 17, 17, 55, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if got := remindersIn(w.fake.to(group)); len(got) != 0 {
		t.Fatalf("reminder before 18h: %+v", got)
	}

	w.clock = time.Date(2026, 11, 17, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	w.bot.Tick(ctx)
	got := remindersIn(w.fake.to(group))
	if len(got) != 1 {
		t.Fatalf("group got %d reminders, want 1", len(got))
	}
	text := got[0].text
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	for _, want := range []string{
		"Escala do *Louvor* de quinta:\n*Culto de quinta*, 19/11, às 19h30",
		"• Violão: @" + w.phone("Líder"),
		"• Baixo: @" + w.phone(bassist),
		"• Bateria: @" + w.phone("Baterista"),
		"é só mudar sua resposta na enquete de confirmação",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("reminder misses %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Falta") || strings.Contains(text, "22/11") {
		t.Fatalf("reminder should only cover a full Thursday:\n%s", text)
	}
	if containsText(w.fake.to(w.phone(bassist)), "Você está na escala de quinta") {
		t.Fatal("a ministry with a group should not also remind in private")
	}
}

func TestTheReminderLeavesOutWhoCannotGoAndNamesTheEmptyRoles(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "😕 Não consigo na quinta (19/11)", false)
	drums := w.slotOf("2026-11-19", "Bateria")
	if _, err := w.store.SetAssignment(ctx, w.ministry.ID, w.month, drums.AssignmentID, nil, w.clock); err != nil {
		t.Fatal(err)
	}

	w.clock = time.Date(2026, 11, 17, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	got := remindersIn(w.fake.to(group))
	if len(got) != 1 {
		t.Fatalf("group got %d reminders, want 1", len(got))
	}
	text := got[0].text
	if !strings.Contains(text, "⚠️ Falta: Baixo e Bateria") || strings.Contains(text, w.phone(bassist)) {
		t.Fatalf("reminder should list the declined bass and the empty drums as missing:\n%s", text)
	}
}

func TestAReminderMissedWhileDownGoesOutUntilTheServiceStarts(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	if got := remindersIn(w.fake.to(group)); len(got) != 1 || !strings.Contains(got[0].text, "19/11") {
		t.Fatalf("the late reminder should still go out on the day: %+v", got)
	}

	late := newWorld(t)
	late.clock = time.Date(2026, 11, 19, 19, 30, 0, 0, saoPaulo)
	late.bot.Tick(context.Background())
	if got := remindersIn(late.fake.to(group)); len(got) != 0 {
		t.Fatalf("reminder after the service started: %+v", got)
	}
}

func TestTheReminderMarksTheLordsSupper(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 13, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	got := remindersIn(w.fake.to(group))
	if len(got) != 1 || !strings.Contains(got[0].text, "15/11") || !strings.Contains(got[0].text, "🍷 Dia de Santa Ceia") {
		t.Fatalf("the third Sunday is a Lord's Supper day: %+v", got)
	}
}

func TestTheReminderFindsServicesInTheNeighbouringMonthsSchedules(t *testing.T) {
	w := newWorld(t)
	w.publish("2026-12")
	w.clock = time.Date(2027, 1, 1, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	if got := remindersIn(w.fake.to(group)); len(got) != 1 || !strings.Contains(got[0].text, "*Culto de domingo*, 03/01") {
		t.Fatalf("Sunday 3/1 sits in December's schedule and should still be reminded: %+v", got)
	}

	w.publish("2027-04")
	w.clock = time.Date(2027, 3, 30, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	if got := remindersIn(w.fake.to(group)); len(got) != 2 || !strings.Contains(got[1].text, "*Culto de quinta*, 01/04") {
		t.Fatalf("Thursday 1/4 is two days into April's schedule and should be reminded in March: %+v", got)
	}
}

func TestAMinistryWithoutAGroupRemindsEachPersonInPrivate(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	if err := w.store.UpdateMinistry(ctx, w.ministry.ID, store.MinistryInput{Name: "Louvor"}); err != nil {
		t.Fatal(err)
	}
	w.clock = time.Date(2026, 11, 17, 18, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	w.bot.Tick(ctx)

	if got := remindersIn(w.fake.to(group)); len(got) != 0 {
		t.Fatalf("no group, yet it got %+v", got)
	}
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	want := "🎶 *Você está na escala de quinta, " + bassist + "!*\n• Quinta, 19/11, às 19h30 · Baixo"
	var reminders int
	for _, m := range w.fake.to(w.phone(bassist)) {
		if strings.HasPrefix(m.text, want) {
			reminders++
		}
	}
	if reminders != 1 {
		t.Fatalf("bassist got %d private reminders, want 1: %+v", reminders, w.fake.to(w.phone(bassist)))
	}
	if !containsText(w.fake.to(w.phone("Líder")), "• Quinta, 19/11, às 19h30 · Voz e Violão") {
		t.Fatalf("one reminder per service lists every role: %+v", w.fake.to(w.phone("Líder")))
	}
}
