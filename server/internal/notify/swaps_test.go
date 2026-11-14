package notify_test

import (
	"context"
	"strings"
	"testing"

	"louvadeira/internal/store"
)

func polls(messages []message) []message {
	var out []message
	for _, m := range messages {
		if m.options != nil {
			out = append(out, m)
		}
	}
	return out
}

func TestDecliningTheWholeWeekSendsOneInviteWithBothServices(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.fake.reset()

	w.vote(poll, "😕 Não consigo em nenhum", false)
	invites := polls(w.fake.to(w.phone(other)))
	if len(invites) != 1 {
		t.Fatalf("%s got %d invites, want one for both services", other, len(invites))
	}
	invite := invites[0]
	want := []string{"✅ Quinta, 19/11", "✅ Domingo, 22/11", "😕 Nenhum dessa vez"}
	if !invite.multiple || strings.Join(invite.options, "|") != strings.Join(want, "|") {
		t.Fatalf("invite multiple=%v options=%v", invite.multiple, invite.options)
	}
	leader := w.fake.to(w.phone("Líder"))
	if len(leader) != 1 || !strings.Contains(leader[0].text, "• quinta, 19/11, às 19h30 (Baixo) — chamei "+other) ||
		!strings.Contains(leader[0].text, "• domingo, 22/11, às 19h (Baixo) — chamei "+other) {
		t.Fatalf("leader messages = %+v", leader)
	}

	w.fake.reset()
	w.tick(invite, false, "✅ Quinta, 19/11", "✅ Domingo, 22/11")
	for _, date := range []string{"2026-11-19", "2026-11-22"} {
		if slot := w.slotOf(date, "Baixo"); slot.PersonName != other || slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
			t.Fatalf("%s slot = %+v", date, slot)
		}
	}
	both := "quinta, 19/11, às 19h30 (Baixo) e domingo, 22/11, às 19h (Baixo)"
	if absent := w.fake.to(w.phone(bassist)); len(absent) != 1 || !strings.Contains(absent[0].text, other+" vai ficar no seu lugar: "+both) {
		t.Fatalf("absent messages = %+v", absent)
	}
	if swaps := w.fake.to(group); len(swaps) != 1 || !strings.Contains(swaps[0].text, other+" substitui "+bassist+" — "+both) {
		t.Fatalf("group messages = %+v", swaps)
	}
}

func TestAnUntickedServiceCountsAsCannot(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "😕 Não consigo em nenhum", false)
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	w.fake.reset()

	w.tick(invite, false, "✅ Quinta, 19/11")
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.PersonName != other {
		t.Fatalf("thursday = %+v", slot)
	}
	if slot := w.slotOf("2026-11-22", "Baixo"); slot.PersonName != bassist || slot.CoverOpen {
		t.Fatalf("sunday = %+v", slot)
	}
	if !containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir "+bassist+" no Louvor: domingo, 22/11, às 19h (Baixo)") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}

	w.fake.reset()
	w.tick(invite, false, "✅ Quinta, 19/11")
	if len(w.fake.sent) != 0 {
		t.Fatalf("the same vote again sent %+v", w.fake.sent)
	}
}

func TestAbsenceOpensSwapsForEveryServiceInThePeriod(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	var singer store.Person
	var held []store.AssignmentDetail
	for _, name := range []string{"Ana", "Bia", "Cris", "Duda"} {
		slots, err := w.store.PersonAssignments(ctx, w.people[name].ID, "2026-11-14", "2026-11-30")
		if err != nil {
			t.Fatal(err)
		}
		if len(slots) > len(held) {
			singer, held = w.people[name], slots
		}
	}
	if len(held) < 2 {
		t.Fatalf("no singer with two services in the period")
	}
	absence, err := w.store.CreateUnavailability(ctx, store.UnavailabilityInput{
		PersonID: singer.ID, StartDate: "2026-11-14", EndDate: "2026-11-30", Reason: store.ReasonHealth,
	})
	if err != nil {
		t.Fatal(err)
	}

	requests, err := w.bot.DeclinePeriod(ctx, absence)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != len(held) {
		t.Fatalf("opened %d swaps for %d services", len(requests), len(held))
	}
	invited := map[string]int{}
	for _, m := range w.fake.sent {
		if m.options == nil {
			continue
		}
		invited[m.to]++
		if !strings.Contains(m.text, singer.Name+" não vai poder servir no Louvor de 14/11 a 30/11") || strings.Contains(m.text, "saúde") {
			t.Fatalf("invite = %q", m.text)
		}
	}
	for phone, n := range invited {
		if n != 1 {
			t.Fatalf("%s got %d invites, want one", phone, n)
		}
	}
	leader := w.fake.to(w.phone("Líder"))
	if len(leader) != 1 || !strings.Contains(leader[0].text, "de 14/11 a 30/11 (saúde):") || strings.Count(leader[0].text, "\n• ") != len(held) {
		t.Fatalf("leader = %+v", leader)
	}
	if left, _ := w.bot.AffectedBy(ctx, absence); len(left) != 0 {
		t.Fatalf("still affected after opening swaps: %+v", left)
	}
}
