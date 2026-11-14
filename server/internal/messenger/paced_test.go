package messenger_test

import (
	"context"
	"testing"
	"time"

	"louvadeira/internal/messenger"
)

func TestPacedSpacesOutMessages(t *testing.T) {
	inner := &recorder{}
	paced := messenger.Paced(inner, 40*time.Millisecond, 60*time.Millisecond)
	start := time.Now()
	for range 3 {
		if _, err := paced.SendText(context.Background(), "5511988887777", "oi"); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 80*time.Millisecond {
		t.Fatalf("three messages took %v, want at least two pauses", elapsed)
	}
	if len(inner.sent) != 3 {
		t.Fatalf("sent %d", len(inner.sent))
	}
}

func TestPacedGivesUpWhenTheCallerDoes(t *testing.T) {
	paced := messenger.Paced(&recorder{}, time.Hour, time.Hour)
	_, _ = paced.SendText(context.Background(), "5511988887777", "oi")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := paced.SendText(ctx, "5511988887777", "oi"); err == nil {
		t.Fatal("waited past the caller's deadline")
	}
}
