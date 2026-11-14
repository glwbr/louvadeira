package notify_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func availabilityPoll(t *testing.T, w *world, name string) message {
	t.Helper()
	for _, m := range w.fake.to(w.phone(name)) {
		if m.options != nil && strings.Contains(m.text, "escala de dezembro do Louvor") {
			return m
		}
	}
	t.Fatalf("%s got no availability poll", name)
	return message{}
}

func TestMonthlyRoundAsksRemindsAndProposesTheSchedule(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.clock = time.Date(2026, 11, 19, 12, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if containsText(w.fake.sent, "escala de dezembro") {
		t.Fatal("asked before day 20")
	}

	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	ana := availabilityPoll(t, w, "Ana")
	if !ana.multiple || ana.options[0] != "Quinta, 03/12" || ana.options[len(ana.options)-1] != "✅ Posso em todos" {
		t.Fatalf("poll = %+v", ana)
	}
	for name := range w.people {
		availabilityPoll(t, w, name)
	}
	bia := availabilityPoll(t, w, "Bia")

	w.fake.reset()
	w.tick(ana, false, "Quinta, 03/12", "Domingo, 06/12")
	if !containsText(w.fake.to(w.phone("Ana")), "Você não pode: quinta, 03/12 e domingo, 06/12") {
		t.Fatalf("ana reply = %+v", w.fake.to(w.phone("Ana")))
	}
	w.tick(ana, false, "Quinta, 03/12")
	absences, _ := w.store.ListUnavailabilities(ctx, "2026-12-01", "2026-12-31")
	if len(absences) != 1 || absences[0].StartDate != "2026-12-03" || !absences[0].FromPoll {
		t.Fatalf("absences after changing the answer = %+v", absences)
	}
	w.tick(bia, false, "✅ Posso em todos")
	if !containsText(w.fake.to(w.phone("Bia")), "Você pode em todos") {
		t.Fatalf("bia reply = %+v", w.fake.to(w.phone("Bia")))
	}

	w.fake.reset()
	w.clock = time.Date(2026, 11, 22, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if !containsText(w.fake.to(w.phone("Cris")), "Ainda falta você responder") || containsText(w.fake.to(w.phone("Ana")), "Ainda falta") {
		t.Fatalf("reminders: cris %+v, ana %+v", w.fake.to(w.phone("Cris")), w.fake.to(w.phone("Ana")))
	}

	w.fake.reset()
	w.clock = time.Date(2026, 11, 23, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if status, _ := w.store.ScheduleStatus(ctx, w.ministry.ID, "2026-12"); status != "draft" {
		t.Fatalf("december = %q", status)
	}
	view, _ := w.store.ScheduleView(ctx, w.ministry.ID, "2026-12")
	for _, sv := range view.Weeks[0].Services {
		for _, role := range sv.Roles {
			for _, slot := range role.Slots {
				if sv.Date == "2026-12-03" && slot.PersonID == w.people["Ana"].ID {
					t.Fatal("Ana is on the Thursday she marked")
				}
			}
		}
	}
	leader := w.fake.to(w.phone("Líder"))
	if !containsText(leader, "Montei a escala de dezembro do Louvor com as respostas da enquete (2 de 8 responderam)") {
		t.Fatalf("leader = %+v", leader)
	}
	approval := lastPoll(t, leader)

	w.fake.reset()
	w.vote(approval, "✅ Pode publicar", false)
	if status, _ := w.store.ScheduleStatus(ctx, w.ministry.ID, "2026-12"); status != "published" {
		t.Fatalf("december after approval = %q", status)
	}
	announcement := w.fake.to(group)
	if len(announcement) != 1 || !strings.Contains(announcement[0].text, "Escala do *Louvor* de dezembro") || !strings.Contains(announcement[0].text, "@55") {
		t.Fatalf("group = %+v", announcement)
	}
	if !containsText(w.fake.to(w.phone("Líder")), "Publicada!") {
		t.Fatalf("leader reply = %+v", w.fake.to(w.phone("Líder")))
	}

	w.fake.reset()
	w.clock = time.Date(2026, 11, 23, 15, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if containsText(w.fake.sent, "Montei a escala") || containsText(w.fake.sent, "escala de dezembro do Louvor") {
		t.Fatalf("the round ran again after publishing: %+v", w.fake.sent)
	}
}
