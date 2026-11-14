package notify_test

import (
	"context"
	"strings"
	"testing"

	"louvadeira/internal/messenger"
	"louvadeira/internal/store"
)

func (w *world) says(name, text string) {
	w.bot.HandleText(context.Background(), messenger.Incoming{From: messenger.Address(w.phone(name)), Text: text})
}

func TestOnlyCommandsFromTheTeamGetAnAnswer(t *testing.T) {
	w := newWorld(t)
	w.bot.HandleText(context.Background(), messenger.Incoming{From: "5511977776666", Text: "escala"})
	w.says("Ana", "Oi, tudo bem? Que horas é o ensaio?")
	if len(w.fake.sent) != 0 {
		t.Fatalf("answered outside the commands: %+v", w.fake.sent)
	}

	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.says(bassist, "Escala")
	replies := w.fake.to(w.phone(bassist))
	if len(replies) != 1 || !strings.Contains(replies[0].text, "Suas próximas escalas") ||
		!strings.Contains(replies[0].text, "• Quinta, 19/11, às 19h30 · Baixo · ⏳ falta confirmar") {
		t.Fatalf("schedule reply = %+v", replies)
	}
}

func TestCannotOnADayAsksFirstAndThenFindsASubstitute(t *testing.T) {
	w := newWorld(t)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]

	w.says(bassist, "Não posso dia 19")
	poll := lastPoll(t, w.fake.to(w.phone(bassist)))
	if !strings.Contains(poll.text, "você não vai poder em 19/11?") || !strings.Contains(poll.text, "• Quinta, 19/11, às 19h30 · Baixo") {
		t.Fatalf("question = %q", poll.text)
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("changed before the answer: %+v", slot)
	}

	w.fake.reset()
	w.vote(poll, "😕 Isso, pode avisar", false)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentDeclined || !slot.CoverOpen {
		t.Fatalf("slot = %+v", slot)
	}
	if len(polls(w.fake.to(w.phone(other)))) != 1 || !containsText(w.fake.to(w.phone(bassist)), "chamei "+other) {
		t.Fatalf("messages = %+v", w.fake.sent)
	}
	absences, _ := w.store.ListUnavailabilities(context.Background(), "2026-11-19", "2026-11-19")
	if len(absences) != 1 || absences[0].PersonName != bassist {
		t.Fatalf("absences = %+v", absences)
	}

	w.fake.reset()
	w.vote(poll, "✅ Me enganei", false)
	if !containsText(w.fake.to(w.phone(bassist)), "já tinha avisado") {
		t.Fatalf("late change of mind = %+v", w.fake.sent)
	}
}

func TestAMistakeChangesNothing(t *testing.T) {
	w := newWorld(t)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.says(bassist, "não posso quinta")
	poll := lastPoll(t, w.fake.sent)
	w.fake.reset()
	w.vote(poll, "✅ Me enganei", false)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("slot = %+v", slot)
	}
	if !containsText(w.fake.sent, "nada mudou") {
		t.Fatalf("reply = %+v", w.fake.sent)
	}
	if absences, _ := w.store.ListUnavailabilities(context.Background(), "2026-11-01", "2026-11-30"); len(absences) != 0 {
		t.Fatalf("absences = %+v", absences)
	}
}

func TestCannotWithoutADateAsksWhichService(t *testing.T) {
	w := newWorld(t)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.says(bassist, "não posso")
	poll := lastPoll(t, w.fake.to(w.phone(bassist)))
	if !poll.multiple || !strings.Contains(strings.Join(poll.options, "|"), "😕 Quinta, 19/11|😕 Domingo, 22/11") ||
		poll.options[len(poll.options)-1] != "✅ Me enganei, vou em todos" {
		t.Fatalf("poll = %+v", poll)
	}
	w.vote(poll, "😕 Domingo, 22/11", false)
	if slot := w.slotOf("2026-11-22", "Baixo"); slot.Status != store.AssignmentDeclined {
		t.Fatalf("sunday = %+v", slot)
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("thursday = %+v", slot)
	}
}

func TestSeveralDaysLetThePersonTickOnlyTheRightOnes(t *testing.T) {
	w := newWorld(t)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.says(bassist, "não posso dia 22, 19 e 15")
	poll := lastPoll(t, w.fake.to(w.phone(bassist)))
	want := "😕 Domingo, 15/11|😕 Quinta, 19/11|😕 Domingo, 22/11|✅ Me enganei"
	if !poll.multiple || strings.Join(poll.options, "|") != want {
		t.Fatalf("poll = %+v", poll)
	}
	w.tick(poll, false, "😕 Quinta, 19/11")
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentDeclined {
		t.Fatalf("thursday = %+v", slot)
	}
	if slot := w.slotOf("2026-11-22", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("sunday was not ticked = %+v", slot)
	}
}

func TestAnAbsenceWithoutServicesIsNotedForTheLeaders(t *testing.T) {
	w := newWorld(t)
	w.says("Ana", "não vou poder de 10/12 a 20/12")
	w.vote(lastPoll(t, w.fake.to(w.phone("Ana"))), "😕 Isso, pode avisar", false)
	if !containsText(w.fake.to(w.phone("Ana")), "Anotado*, Ana! A gente não coloca você na escala de 10/12 a 20/12") {
		t.Fatalf("ana = %+v", w.fake.to(w.phone("Ana")))
	}
	if !containsText(w.fake.to(w.phone("Líder")), "📅 *Ana* avisou que não vai poder de 10/12 a 20/12.") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
}

func TestShortTextRepliesAnswerTheLatestPoll(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	sunday := w.slotOf("2026-11-15", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[sunday]

	w.says(sunday, "ok")
	if slot := w.slotOf("2026-11-15", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("\"ok\" answered the poll: %+v", slot)
	}
	w.says(sunday, "Não vou")
	if slot := w.slotOf("2026-11-15", "Baixo"); slot.Status != store.AssignmentDeclined || !slot.CoverOpen {
		t.Fatalf("after \"não vou\": %+v", slot)
	}
	w.says(other, "Consigo!")
	if slot := w.slotOf("2026-11-15", "Baixo"); slot.PersonName != other || slot.Status != store.AssignmentConfirmed {
		t.Fatalf("after \"consigo\": %+v", slot)
	}
}

func TestShortTextRepliesOnAWeekWithTwoServices(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	thursday := w.slotOf("2026-11-19", "Baixo").PersonName
	w.fake.reset()
	w.says(thursday, "não")
	if !containsText(w.fake.to(w.phone(thursday)), "é só marcar na enquete") || w.slotOf("2026-11-19", "Baixo").Status != store.AssignmentScheduled {
		t.Fatalf("a plain no for a week with two services = %+v", w.fake.sent)
	}
	w.says(thursday, "sim")
	for _, date := range []string{"2026-11-19", "2026-11-22"} {
		if slot := w.slotOf(date, "Baixo"); slot.Status != store.AssignmentConfirmed {
			t.Fatalf("%s after \"sim\" = %+v", date, slot)
		}
	}
}

func TestALeaderAwayHearsWhatHasNoSubstitute(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ministry, _ := w.store.Ministry(ctx, w.ministry.ID)
	var voice int64
	for _, role := range ministry.Roles {
		if role.Name == "Voz" {
			voice = role.ID
		}
	}
	if err := w.store.PutMember(ctx, w.ministry.ID, w.people["Bia"].ID, store.MemberInput{IsLeader: true,
		Roles: []store.MemberRole{{RoleID: voice}}}); err != nil {
		t.Fatal(err)
	}
	w.says("Líder", "não posso dia 19")
	w.vote(lastPoll(t, w.fake.to(w.phone("Líder"))), "😕 Isso, pode avisar", false)

	reply := w.fake.to(w.phone("Líder"))
	if !containsText(reply, "Ninguém da equipe está livre para:\n• Quinta, 19/11, às 19h30 · Violão") || containsText(reply, "⚠️ *Líder*") {
		t.Fatalf("leader's own messages = %+v", reply)
	}
	if !containsText(w.fake.to(w.phone("Bia")), "• Quinta, 19/11, às 19h30\n   ◦ Voz: chamei") ||
		!containsText(w.fake.to(w.phone("Bia")), "   ◦ Violão: ninguém livre") {
		t.Fatalf("other leader = %+v", w.fake.to(w.phone("Bia")))
	}
}

func TestScheduleOnlyShowsPendingInsideTheConfirmationWeek(t *testing.T) {
	w := newWorld(t)
	w.says("Líder", "escala")
	reply := w.fake.to(w.phone("Líder"))
	if !containsText(reply, "• Quinta, 19/11, às 19h30 · Voz e Violão · ⏳ falta confirmar") ||
		!containsText(reply, "• Domingo, 29/11, às 19h · Voz e Violão\n") {
		t.Fatalf("schedule = %+v", reply)
	}
}
