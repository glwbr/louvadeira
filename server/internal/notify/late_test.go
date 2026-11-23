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

func TestTheAnswerToADeclineNamesWhoIsBeingInvitedBeforeTheyAre(t *testing.T) {
	w := newWorld(t)
	var queued []func()
	w.bot.Async = func(task func()) { queued = append(queued, task) }
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]

	requests, err := w.bot.Decline(context.Background(), []int64{slot.AssignmentID}, false)
	if err != nil || len(requests) != 1 {
		t.Fatalf("requests = %+v, err %v", requests, err)
	}
	if invites := requests[0].Invites; len(invites) != 1 || invites[0].Name != other {
		t.Fatalf("invites = %+v, want %s", invites, other)
	}
	if len(w.fake.sent) != 0 || len(queued) != 1 {
		t.Fatalf("the invitations should still be queued: sent %d, queued %d", len(w.fake.sent), len(queued))
	}
	queued[0]()
	if len(polls(w.fake.to(w.phone(other)))) != 1 {
		t.Fatalf("%s got no invite", other)
	}
}

func TestAnArchivedMinistryGetsNoMoreMessages(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	if err := w.store.ArchiveMinistry(ctx, w.ministry.ID); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	w.bot.Tick(ctx)
	w.vote(invite, "✅ Consigo", false)
	for _, m := range w.fake.sent {
		if m.to != w.phone(other) {
			t.Fatalf("sent %+v for an archived ministry", m)
		}
	}
	if len(polls(w.fake.sent)) != 0 {
		t.Fatalf("asked %+v for an archived ministry", polls(w.fake.sent))
	}
}

func TestSomeoneWhoLeftNoLongerHoldsASwapOrTheirSlots(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	bassist := slot.PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	if err := w.store.ArchivePerson(ctx, w.people[other].ID); err != nil {
		t.Fatal(err)
	}
	w.bot.PersonLeft(ctx, w.people[other].ID, 0)
	if !containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir "+bassist) {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}

	w.fake.reset()
	var voice store.ScheduleSlot
	for _, v := range w.slotsOf("2026-11-22", "Voz") {
		if v.PersonName != "Líder" {
			voice = v
		}
	}
	if err := w.store.RemoveMember(ctx, w.ministry.ID, voice.PersonID); err != nil {
		t.Fatal(err)
	}
	w.bot.PersonLeft(ctx, voice.PersonID, w.ministry.ID)
	for _, v := range w.slotsOf("2026-11-22", "Voz") {
		if v.AssignmentID == voice.AssignmentID && (v.Status != store.AssignmentDeclined || !v.CoverOpen) {
			t.Fatalf("%s's sunday = %+v, want a swap open", voice.PersonName, v)
		}
	}
	if !containsText(w.fake.to(w.phone("Líder")), voice.PersonName+"* não vai poder servir no Louvor daqui para frente") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
}

func TestAnInvitationThatFailedIsRetriedEvenWhenAnotherWentOut(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	var slot store.ScheduleSlot
	for _, v := range w.slotsOf(sunday, "Voz") {
		if v.PersonName != "Líder" {
			slot = v
		}
	}
	free := w.freeVoices()
	unreached, reached := free[0], free[1]
	w.fake.fail = func(m message) error {
		if m.to == w.phone(unreached) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	w.vote(lastPoll(t, w.fake.to(w.phone(reached))), "😕 Dessa vez não", false)
	if containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir") {
		t.Fatal("the swap ended before reaching everyone")
	}

	w.fake.fail = nil
	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	w.bot.Tick(ctx)
	invites := polls(w.fake.to(w.phone(unreached)))
	if len(invites) != 1 {
		t.Fatalf("%s got %d invitations after the retry, want 1", unreached, len(invites))
	}
	w.vote(invites[0], "✅ Consigo", false)
	for _, v := range w.slotsOf(sunday, "Voz") {
		if v.AssignmentID == slot.AssignmentID && v.PersonName != unreached {
			t.Fatalf("slot = %+v, want %s", v, unreached)
		}
	}
}

func TestAnInvitationThatCannotBeSentForADayCountsAsANo(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-22", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	w.fake.fail = func(m message) error {
		if m.to == w.phone(other) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()
	w.clock = w.clock.Add(25 * time.Hour)
	w.bot.Tick(ctx)
	if !containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir "+slot.PersonName) {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
	if s := w.slotOf("2026-11-22", "Baixo"); s.CoverOpen {
		t.Fatalf("slot = %+v, want the swap closed", s)
	}
}

func TestThePendingAlertGoesOutAt9hThreeDaysBefore(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	pending := func() bool {
		for _, m := range w.fake.to(w.phone("Líder")) {
			if strings.Contains(m.text, "Falta responder* sobre domingo, 22/11") {
				return true
			}
		}
		return false
	}
	w.clock = time.Date(2026, 11, 19, 8, 30, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if pending() {
		t.Fatal("alerted before 9h")
	}
	w.clock = time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if !pending() {
		t.Fatalf("no alert at 9h on thursday about sunday: %+v", w.fake.to(w.phone("Líder")))
	}
}

func TestADayOffInThePollKeepsAnAbsenceRegisteredElsewhere(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	trip, err := w.store.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: w.people[bassist].ID, StartDate: "2026-11-22", EndDate: "2026-11-22", Reason: store.ReasonTravel})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.bot.DeclinePeriod(ctx, trip); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	if sunday := w.slotOf("2026-11-22", "Baixo"); sunday.Status != store.AssignmentDeclined || !sunday.CoverOpen {
		t.Fatalf("sunday = %+v, want the trip to stand", sunday)
	}
	if containsText(w.fake.to(w.phone("Líder")), "conseguiu ir, afinal") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
}

func TestAnInvitationStillUnsentWhenTheServicePassesClosesQuietly(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	w.fake.fail = func(m message) error {
		if m.to == w.phone(other) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	w.fake.fail = nil
	w.fake.reset()

	w.clock = time.Date(2026, 11, 20, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(ctx)
	if containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir") {
		t.Fatal("alerted about a service that already happened")
	}
	if polls := pollsAbout(w.fake.to(w.phone(other)), "19/11"); polls != 0 {
		t.Fatalf("%s was invited to a past service", other)
	}
	if s := w.slotOf("2026-11-19", "Baixo"); s.CoverOpen {
		t.Fatalf("slot = %+v, want the swap closed", s)
	}
}

func TestSomeoneNeverReachedIsNotReleasedNorListedAsInvited(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.bot.Tick(ctx)
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	poll := w.fake.sentPollFor(t, w.phone(slot.PersonName), "19/11")
	w.fake.fail = func(m message) error {
		if m.to == w.phone(other) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	w.vote(poll, "😕 Não consigo na quinta (19/11)", false)
	requests, _ := w.store.CoverRequestsForSchedule(ctx, w.ministry.ID, w.month)
	for _, r := range requests {
		if r.Status == store.CoverOpen && len(r.Invites) != 0 {
			t.Fatalf("invites = %+v, want none while the send failed", r.Invites)
		}
	}
	w.fake.fail = nil
	w.fake.reset()

	w.vote(poll, "✅ Vou nos dois", false)
	if len(w.fake.to(w.phone(other))) != 0 {
		t.Fatalf("%s got %+v about an invitation never sent", other, w.fake.to(w.phone(other)))
	}
}

func TestARetryLeavesOutWhoBecameAway(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[slot.PersonName]
	w.fake.fail = func(m message) error {
		if m.to == w.phone(other) {
			return errors.New("websocket EOF")
		}
		return nil
	}
	if _, err := w.bot.Decline(ctx, []int64{slot.AssignmentID}, false); err != nil {
		t.Fatal(err)
	}
	w.fake.fail = nil
	if _, err := w.store.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: w.people[other].ID, StartDate: "2026-11-19", EndDate: "2026-11-19", Reason: store.ReasonTravel}); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	w.clock = w.clock.Add(5 * time.Minute)
	w.bot.Tick(ctx)
	if pollsAbout(w.fake.to(w.phone(other)), "19/11") != 0 {
		t.Fatalf("%s was invited although away", other)
	}
	if !containsText(w.fake.to(w.phone("Líder")), "Ninguém conseguiu substituir") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
}
