package notify_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestASwapAnnouncementThatFailedGoesOutOnTheNextTick(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	w.fake.reset()
	w.fake.fail = func(m message) error {
		if m.to == group {
			return errors.New("websocket EOF")
		}
		return nil
	}
	w.vote(invite, "✅ Consigo", false)
	if len(w.fake.to(group)) != 0 {
		t.Fatal("the failed announcement counted as sent")
	}

	w.fake.fail = nil
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	if posts := w.fake.to(group); len(posts) != 1 || !strings.Contains(posts[0].text, "Mudança na escala") {
		t.Fatalf("group = %+v, want the swap announced once", posts)
	}
}

func TestMessagesToOnePersonKeepTheirOrderAfterAFailure(t *testing.T) {
	w := newWorld(t)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	down := true
	w.fake.fail = func(m message) error {
		if down && m.to == w.phone(bassist) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	w.says(bassist, "ajuda")
	if len(w.fake.to(w.phone(bassist))) != 0 {
		t.Fatal("the failed reply counted as sent")
	}
	down = false
	w.says(bassist, "escala")
	var replies []string
	for _, m := range w.fake.to(w.phone(bassist)) {
		replies = append(replies, m.text)
	}
	if len(replies) != 2 || !strings.Contains(replies[0], "Assistente das escalas") || !strings.Contains(replies[1], "Suas próximas escalas") {
		t.Fatalf("replies = %q, want the help and then the schedule", replies)
	}
}

func TestAMessageStuckForHalfADayIsDropped(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.fake.fail = func(m message) error {
		if m.to == w.phone(bassist) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	w.says(bassist, "ajuda")
	w.fake.fail = nil
	w.clock = w.clock.Add(13 * time.Hour)
	w.bot.Tick(ctx)
	for _, m := range w.fake.to(w.phone(bassist)) {
		if m.options == nil {
			t.Fatalf("sent a 13-hour-old reply: %q", m.text)
		}
	}
}
