package notify_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"louvadeira/internal/store"
)

func TestADeclineThatArrivesAfterTheServiceChangesNothing(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.fake.reset()

	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %+v about a service that already happened", w.fake.sent)
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status == store.AssignmentDeclined || slot.CoverOpen {
		t.Fatalf("slot = %+v", slot)
	}
}

func TestACoverAcceptedAfterTheServiceChangesNothing(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	w.fake.reset()

	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.vote(invite, "✅ Consigo", false)
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %+v about a service that already happened", w.fake.sent)
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.PersonName != bassist {
		t.Fatalf("slot = %+v", slot)
	}
}

func TestAnAdminCannotOpenASwapForAServiceThatHappened(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	_, err := w.bot.Decline(context.Background(), []int64{w.slotOf("2026-11-19", "Baixo").AssignmentID}, false)
	var validation store.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want a validation error", err)
	}
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %+v", w.fake.sent)
	}
}

func TestDecliningAgainAfterComingBackInvitesTheSubstituteAgain(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	w.vote(poll, "✅ Vou nos dois", false)
	if !containsText(w.fake.to(w.phone(other)), "Não precisa mais") {
		t.Fatalf("%s was not released: %+v", other, w.fake.to(w.phone(other)))
	}
	w.fake.reset()

	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	if invites := polls(w.fake.to(w.phone(other))); len(invites) != 1 {
		t.Fatalf("%s got %d new invites, want 1", other, len(invites))
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); !slot.CoverOpen {
		t.Fatalf("slot = %+v", slot)
	}
}
