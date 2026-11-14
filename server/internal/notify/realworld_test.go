package notify_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/notify"
	"louvadeira/internal/store"
)

var errOffline = errors.New("websocket not connected")

func count(messages []message, fragment string) int {
	n := 0
	for _, m := range messages {
		if strings.Contains(m.text, fragment) {
			n++
		}
	}
	return n
}

func TestTheSameAnswerTwiceGetsOneReply(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")

	w.vote(poll, "✅ Vou nos dois", false)
	w.vote(poll, "✅ Vou nos dois", false)
	w.says(bassist, "sim")
	if n := count(w.fake.to(w.phone(bassist)), "Confirmado*"); n != 1 {
		t.Fatalf("got %d confirmations", n)
	}
}

func TestTheSameAvailabilityAnswerIsAcknowledgedOnce(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	poll := availabilityPoll(t, w, "Ana")

	w.tick(poll, false, "Quinta, 03/12")
	w.tick(poll, false, "Quinta, 03/12")
	if n := count(w.fake.to(w.phone("Ana")), "Anotado*"); n != 1 {
		t.Fatalf("got %d acknowledgements for the same answer", n)
	}
	w.tick(poll, false, "Quinta, 03/12", "Domingo, 06/12")
	if n := count(w.fake.to(w.phone("Ana")), "Anotado*"); n != 2 {
		t.Fatalf("a changed answer should be acknowledged again, got %d", n)
	}
}

func TestApprovingTwicePublishesOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	w.clock = time.Date(2026, 11, 23, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	approval := lastPoll(t, w.fake.to(w.phone("Líder")))
	w.fake.reset()

	w.vote(approval, "✅ Pode publicar", false)
	w.vote(approval, "✅ Pode publicar", false)
	if n := len(w.fake.to(group)); n != 1 {
		t.Fatalf("group got %d announcements", n)
	}
	leader := w.fake.to(w.phone("Líder"))
	if count(leader, "Publicada!") != 1 || count(leader, "já estava publicada") != 0 {
		t.Fatalf("leader = %+v", leader)
	}
}

func TestLeadersSeeTheSwapOnlyInTheGroup(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	absent := w.slotOf("2026-11-15", "Baixo").PersonName
	substitute := map[string]string{"Edu": "Fred", "Fred": "Edu"}[absent]
	w.vote(w.fake.sentPollFor(t, w.phone(absent), "15/11"), "😕 Não vou conseguir", false)
	w.vote(lastPoll(t, w.fake.to(w.phone(substitute))), "✅ Consigo", false)

	if n := count(w.fake.to(w.phone("Líder")), "Mudança na escala"); n != 0 {
		t.Fatalf("the leader got the swap %d times in private as well", n)
	}
	if n := count(w.fake.to(group), "Mudança na escala"); n != 1 {
		t.Fatalf("group got %d swap notices", n)
	}
}

func TestRunsAtTheSameTimeAskEachPersonOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); w.bot.Tick(ctx) }()
		go func() { defer wg.Done(); _, _ = w.bot.AskNow(ctx, "2026-11-15", "2026-11-22") }()
	}
	wg.Wait()
	if n := len(polls(w.fake.sent)); n != 10 {
		t.Fatalf("sent %d polls, want one per person and week (10)", n)
	}
}

func TestAFailedSendIsRetriedOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.fake.fail = func(m message) error {
		if m.to == w.phone(bassist) {
			return errOffline
		}
		return nil
	}
	w.bot.Tick(ctx)
	if n := len(polls(w.fake.to(w.phone(bassist)))); n != 0 {
		t.Fatalf("delivered %d polls while offline", n)
	}
	w.fake.fail = nil
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	if n := count(w.fake.to(w.phone(bassist)), "19/11"); n != 1 {
		t.Fatalf("got %d polls for the week after coming back", n)
	}
}

func TestASendThatMayHaveArrivedIsNotRepeated(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.fake.fail = func(m message) error {
		if m.to == w.phone(bassist) && strings.Contains(m.text, "19/11") {
			return messenger.ErrMaybeDelivered
		}
		return nil
	}
	w.bot.Tick(ctx)
	w.fake.fail = nil
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	if n := count(w.fake.to(w.phone(bassist)), "19/11"); n != 1 {
		t.Fatalf("got %d polls; an unconfirmed send must not be repeated", n)
	}
	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "✅ Vou nos dois", false)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentConfirmed {
		t.Fatalf("the answer to an unconfirmed poll was lost: %+v", slot)
	}
}

func TestASplitAvailabilityPollOnlyResendsTheMissingPart(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	tuesday, _ := w.store.CreateService(ctx, store.ServiceInput{Name: "Culto de terça", Weekday: 2, StartTime: "19:30"})
	ministry, _ := w.store.Ministry(ctx, w.ministry.ID)
	if err := w.store.SetMinistryServices(ctx, w.ministry.ID, append(ministry.ServiceIDs, tuesday.ID)); err != nil {
		t.Fatal(err)
	}
	parts := 0
	w.fake.fail = func(m message) error {
		if m.to == w.phone("Ana") && strings.Contains(m.text, "Escala de dezembro") {
			parts++
			if parts == 2 {
				return errOffline
			}
		}
		return nil
	}
	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)

	got := polls(w.fake.to(w.phone("Ana")))
	var availability []message
	for _, m := range got {
		if strings.Contains(m.text, "Escala de dezembro") {
			availability = append(availability, m)
		}
	}
	if len(availability) != 2 || availability[0].options[0] == availability[1].options[0] {
		t.Fatalf("Ana should get each half once, got %d polls", len(availability))
	}
}

func TestInvitesThatCouldNotBeSentAreRetried(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	absent := w.slotOf("2026-11-15", "Baixo").PersonName
	substitute := map[string]string{"Edu": "Fred", "Fred": "Edu"}[absent]
	w.fake.fail = func(m message) error {
		if m.to == w.phone(substitute) && m.options != nil {
			return errOffline
		}
		return nil
	}
	poll := w.fake.sentPollFor(t, w.phone(absent), "15/11")
	w.fake.reset()
	w.vote(poll, "😕 Não vou conseguir", false)
	w.vote(poll, "😕 Não vou conseguir", false)
	if slot := w.slotOf("2026-11-15", "Baixo"); !slot.CoverOpen {
		t.Fatalf("a send failure closed the swap: %+v", slot)
	}
	leader := w.fake.to(w.phone("Líder"))
	if count(leader, "Ninguém da equipe está livre") != 0 || count(leader, "Não consegui avisar "+substitute) != 1 {
		t.Fatalf("leader = %+v", leader)
	}

	w.fake.fail = nil
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	w.bot.Tick(ctx)
	invites := polls(w.fake.to(w.phone(substitute)))
	if len(invites) != 1 {
		t.Fatalf("substitute got %d invitations after the retry", len(invites))
	}
	w.vote(invites[0], "✅ Consigo", false)
	if slot := w.slotOf("2026-11-15", "Baixo"); slot.PersonName != substitute {
		t.Fatalf("slot after the retried invitation = %+v", slot)
	}
}

func TestAVoteSurvivesARestart(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.bot.Settle = time.Hour
	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "✅ Vou nos dois", false)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("applied before settling: %+v", slot)
	}

	restarted := &notify.Bot{Store: w.store, Messenger: w.fake, Log: slog.New(slog.NewTextHandler(logOutput, nil)),
		Location: saoPaulo, Now: func() time.Time { return w.clock }}
	restarted.ReplayPendingVotes(ctx)
	restarted.ReplayPendingVotes(ctx)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentConfirmed {
		t.Fatalf("the vote was lost in the restart: %+v", slot)
	}
	if n := count(w.fake.to(w.phone(bassist)), "Confirmado*"); n != 1 {
		t.Fatalf("got %d confirmations", n)
	}
}

func TestTextsThatArriveLateAfterDowntime(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.clock = time.Date(2026, 11, 19, 0, 10, 0, 0, saoPaulo)

	w.bot.HandleText(ctx, messenger.Incoming{From: messenger.Address(w.phone(bassist)), Text: "escala", Time: w.clock.Add(-3 * time.Hour)})
	if len(w.fake.sent) != 0 {
		t.Fatalf("answered a 3-hour-old message: %+v", w.fake.sent)
	}
	w.bot.HandleText(ctx, messenger.Incoming{From: messenger.Address(w.phone(bassist)), Text: "não posso amanhã", Time: w.clock.Add(-100 * time.Minute)})
	poll := lastPoll(t, w.fake.to(w.phone(bassist)))
	if !strings.Contains(poll.text, "você não vai poder em 19/11?") {
		t.Fatalf("\"amanhã\" must be counted from when it was sent: %q", poll.text)
	}
}
