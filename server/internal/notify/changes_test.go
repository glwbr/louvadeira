package notify_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"louvadeira/internal/store"
)

/**
 * NOTE: A Sunday voice that is not the leader's, with the week already asked about, as when a leader opens the app to
 * change it.
 */
func (w *world) aSundayVoice() store.ScheduleSlot {
	w.t.Helper()
	w.tickAt(time.Date(2026, 11, 16, 12, 0, 0, 0, saoPaulo))
	for _, slot := range w.slotsOf(sunday, "Voz") {
		if slot.PersonName != "Líder" {
			return slot
		}
	}
	w.t.Fatal("no voice to change")
	return store.ScheduleSlot{}
}

func (w *world) change(slot store.ScheduleSlot, name string) {
	w.t.Helper()
	var personID *int64
	if name != "" {
		id := w.people[name].ID
		personID = &id
	}
	change, err := w.store.SetAssignment(context.Background(), w.ministry.ID, w.month, slot.AssignmentID, personID, w.clock)
	if err != nil {
		w.t.Fatal(err)
	}
	w.bot.AnnounceChange(context.Background(), change, w.people["Líder"].ID)
}

func pollsAbout(messages []message, date string) int {
	count := 0
	for _, m := range messages {
		if m.options != nil && strings.Contains(m.text, date) {
			count++
		}
	}
	return count
}

func TestALeaderChangeTellsTheGroupAndWhoLeft(t *testing.T) {
	w := newWorld(t)
	slot := w.aSundayVoice()
	substitute := w.freeVoices()[0]
	w.fake.reset()

	w.change(slot, substitute)

	want := "🔄 *Mudança na escala do Louvor*\n@" + w.phone(substitute) + " substitui @" + w.phone(slot.PersonName) +
		"\n• Domingo, 22/11, às 19h · Voz"
	if group := w.fake.to(group); len(group) != 1 || group[0].text != want {
		t.Fatalf("group = %+v", group)
	}
	if !containsText(w.fake.to(w.phone(slot.PersonName)), "Mudança na escala: "+substitute+" fica no seu lugar.\n• Domingo, 22/11") {
		t.Fatalf("who left = %+v", w.fake.to(w.phone(slot.PersonName)))
	}
	if leader := w.fake.to(w.phone("Líder")); len(leader) != 0 {
		t.Fatalf("the leader who made the change was told: %+v", leader)
	}

	w.tickAt(time.Date(2026, 11, 16, 12, 5, 0, 0, saoPaulo))
	if n := pollsAbout(w.fake.to(w.phone(substitute)), "22/11"); n != 1 {
		t.Fatalf("%s was asked about Sunday %d times, want once: %+v", substitute, n, w.fake.to(w.phone(substitute)))
	}
}

func TestALeaderChangeSettlesTheSwapOfWhoCouldNotGo(t *testing.T) {
	w := newWorld(t)
	slot := w.aSundayVoice()
	if _, err := w.bot.Decline(context.Background(), []int64{slot.AssignmentID}, true); err != nil {
		t.Fatal(err)
	}
	free := w.freeVoices()
	if len(free) < 2 {
		t.Fatalf("free voices = %v", free)
	}
	lateInvite := w.fake.sentPollFor(t, w.phone(free[1]), "22/11")
	w.fake.reset()

	w.change(slot, free[0])

	if !containsText(w.fake.to(w.phone(slot.PersonName)), "*Boa notícia*, "+slot.PersonName+"! "+free[0]+" vai ficar no seu lugar") {
		t.Fatalf("who left = %+v", w.fake.to(w.phone(slot.PersonName)))
	}
	if !containsText(w.fake.to(w.phone(free[1])), free[0]+" já vai substituir") {
		t.Fatalf("the other invitee = %+v", w.fake.to(w.phone(free[1])))
	}
	if containsText(w.fake.to(w.phone(free[0])), "já vai substituir") {
		t.Fatalf("the substitute heard someone else took it: %+v", w.fake.to(w.phone(free[0])))
	}
	if !containsText(w.fake.to(group), "@"+w.phone(free[0])+" substitui @"+w.phone(slot.PersonName)) {
		t.Fatalf("group = %+v", w.fake.to(group))
	}
	if covers, _ := w.store.CoverRequestsForSchedule(context.Background(), w.ministry.ID, w.month); len(covers) != 1 || covers[0].Status != store.CoverCovered {
		t.Fatalf("covers = %+v", covers)
	}

	w.fake.reset()
	w.vote(lateInvite, "✅ Consigo", false)
	if replies := w.fake.to(w.phone(free[1])); len(replies) != 1 || replies[0].text != "Obrigado pela disposição! 🙏 Esse já foi resolvido." {
		t.Fatalf("late volunteer = %+v", replies)
	}
	if len(w.fake.sent) != 1 {
		t.Fatalf("a late vote told others: %+v", w.fake.sent)
	}
	if now := w.voiceOf(free[0]); now.AssignmentID != slot.AssignmentID || now.Status != store.AssignmentScheduled {
		t.Fatalf("the late vote moved the place: %+v", now)
	}
	if late := w.voiceOf(free[1]); late.AssignmentID != 0 {
		t.Fatalf("the late volunteer got a place: %+v", late)
	}
}

func TestEmptyingAPlaceOnlyTellsWhoLeft(t *testing.T) {
	w := newWorld(t)
	slot := w.aSundayVoice()
	w.fake.reset()

	w.change(slot, "")

	if !containsText(w.fake.to(w.phone(slot.PersonName)), "Mudança na escala: você saiu deste culto.\n• Domingo, 22/11") {
		t.Fatalf("who left = %+v", w.fake.to(w.phone(slot.PersonName)))
	}
	if len(w.fake.sent) != 1 {
		t.Fatalf("sent = %+v", w.fake.sent)
	}
}

func TestChangesToPastServicesStayQuiet(t *testing.T) {
	w := newWorld(t)
	slot := w.aSundayVoice()
	w.clock = time.Date(2026, 11, 23, 12, 0, 0, 0, saoPaulo)
	w.fake.reset()

	w.change(slot, w.freeVoices()[0])

	if len(w.fake.sent) != 0 {
		t.Fatalf("sent = %+v", w.fake.sent)
	}
}
