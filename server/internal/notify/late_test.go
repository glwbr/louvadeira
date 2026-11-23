package notify_test

import (
	"context"
	"errors"
	"strings"
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

func TestSwitchingTheDayOffTakesTheFirstDayBack(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	w.vote(poll, "😕 Não consigo no domingo (22/11)", false)

	if slot := w.slotOf("2026-11-19", "Baixo"); slot.PersonName != bassist || slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
		t.Fatalf("thursday = %+v, want the bassist back and confirmed", slot)
	}
	if slot := w.slotOf("2026-11-22", "Baixo"); slot.Status != store.AssignmentDeclined || !slot.CoverOpen {
		t.Fatalf("sunday = %+v, want declined with a substitute sought", slot)
	}
}

func TestAPendingAlertThatFailedToSendGoesOutOnTheNextTick(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	leader := w.phone("Líder")
	w.fake.reset()
	w.fake.fail = func(m message) error {
		if m.to == leader {
			return errors.New("websocket EOF")
		}
		return nil
	}
	w.clock = time.Date(2026, 11, 16, 20, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)

	w.fake.fail = nil
	for _, minute := range []int{5, 10} {
		w.clock = time.Date(2026, 11, 16, 20, minute, 0, 0, saoPaulo)
		w.bot.Tick(ctx)
	}
	alerts := 0
	for _, m := range w.fake.to(leader) {
		if strings.Contains(m.text, "Falta responder") && strings.Contains(m.text, "19/11") {
			alerts++
		}
	}
	if alerts != 1 {
		t.Fatalf("leader got %d alerts about 19/11, want 1: %+v", alerts, w.fake.to(leader))
	}
}
