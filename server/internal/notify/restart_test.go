package notify_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"louvadeira/internal/store"
)

func TestAStopDuringASendStartsNoOtherAndTheNextStartSendsThemAll(t *testing.T) {
	w := newWorld(t)
	ctx, stop := context.WithCancel(context.Background())
	w.fake.fail = func(message) error {
		stop()
		return context.Canceled
	}
	w.bot.Tick(ctx)
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %d messages after the stop", len(w.fake.sent))
	}

	w.fake.fail = nil
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 10 {
		t.Fatalf("sent %d polls, want 5 people x 2 weeks, including the one cut short", len(w.fake.sent))
	}
}

func TestAMessageCutShortByAPowerCutGoesOutAtTheNextStart(t *testing.T) {
	w := newWorld(t)
	var cut string
	w.fake.fail = func(m message) error {
		cut = m.to
		runtime.Goexit()
		return nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.bot.Tick(context.Background())
	}()
	<-done

	w.fake.fail = nil
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 9 {
		t.Fatalf("sent %d polls, want all 10 but the one cut short while its claim stands", len(w.fake.sent))
	}

	dropped, err := w.store.DropUnsentClaims(context.Background())
	if err != nil || dropped != 1 {
		t.Fatalf("dropped %d claims, err %v; want 1", dropped, err)
	}
	w.fake.reset()
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 1 || len(w.fake.to(cut)) != 1 {
		t.Fatalf("sent %d polls after the start, want only the one cut short", len(w.fake.sent))
	}
}

func TestNothingScheduledGoesOutWhileTheClockIsUnsynced(t *testing.T) {
	w := newWorld(t)
	synced := false
	w.bot.ClockSynced = func() bool { return synced }
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %d messages with the clock unsynced", len(w.fake.sent))
	}
	synced = true
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 10 {
		t.Fatalf("sent %d polls once synced, want 10", len(w.fake.sent))
	}
}

func TestVotesAndTextsWaitForTheClock(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.fake.reset()
	var synced atomic.Bool
	w.bot.ClockSynced = synced.Load
	w.bot.Settle = 20 * time.Millisecond

	w.vote(poll, "✅ Vou nos dois", false)
	w.says(bassist, "ajuda")
	time.Sleep(150 * time.Millisecond)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled || len(w.fake.sent) != 0 {
		t.Fatalf("acted with the clock unsynced: slot %+v, sent %+v", slot, w.fake.sent)
	}

	synced.Store(true)
	time.Sleep(150 * time.Millisecond)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentConfirmed {
		t.Fatalf("slot = %+v after the clock synced", slot)
	}
	if !containsText(w.fake.to(w.phone(bassist)), "Assistente das escalas") {
		t.Fatalf("the help was not sent after the clock synced: %+v", w.fake.to(w.phone(bassist)))
	}
}
