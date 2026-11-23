package notify_test

import (
	"context"
	"runtime"
	"testing"
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
