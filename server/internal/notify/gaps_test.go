package notify_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"louvadeira/internal/store"
)

const sunday = "2026-11-22"

func (w *world) slotsOf(date, role string) []store.ScheduleSlot {
	w.t.Helper()
	view, err := w.store.ScheduleView(context.Background(), w.ministry.ID, w.month)
	if err != nil {
		w.t.Fatal(err)
	}
	for _, week := range view.Weeks {
		for _, sv := range week.Services {
			if sv.Date != date {
				continue
			}
			for _, r := range sv.Roles {
				if r.Name == role {
					return r.Slots
				}
			}
		}
	}
	w.t.Fatalf("no %s slot on %s", role, date)
	return nil
}

/**
 * NOTE: Empties one Voz place on Sunday that is not the leader's, the way a leader leaves it empty in the app.
 */
func (w *world) emptyAVoice() string {
	w.t.Helper()
	for _, slot := range w.slotsOf(sunday, "Voz") {
		if slot.PersonName != "Líder" {
			if err := w.store.SetAssignment(context.Background(), w.ministry.ID, w.month, slot.AssignmentID, nil, w.clock); err != nil {
				w.t.Fatal(err)
			}
			return slot.PersonName
		}
	}
	w.t.Fatal("no voice to empty")
	return ""
}

func gapPolls(messages []message) []message {
	var out []message
	for _, m := range messages {
		if m.options != nil && strings.Contains(m.text, "Ainda falta gente") {
			out = append(out, m)
		}
	}
	return out
}

func (w *world) tickAt(clock time.Time) {
	w.clock = clock
	w.bot.Tick(context.Background())
}

func TestLeadersHearAboutAShortServiceThreeDaysBefore(t *testing.T) {
	w := newWorld(t)
	w.emptyAVoice()

	w.tickAt(time.Date(2026, 11, 18, 12, 0, 0, 0, saoPaulo))
	w.tickAt(time.Date(2026, 11, 19, 8, 30, 0, 0, saoPaulo))
	if polls := gapPolls(w.fake.to(w.phone("Líder"))); len(polls) != 0 {
		t.Fatalf("asked too early: %+v", polls)
	}

	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	w.tickAt(time.Date(2026, 11, 19, 15, 0, 0, 0, saoPaulo))
	polls := gapPolls(w.fake.to(w.phone("Líder")))
	if len(polls) != 1 {
		t.Fatalf("got %d questions, want one", len(polls))
	}
	poll := polls[0]
	if !strings.HasPrefix(poll.text, "⚠️ Louvor · Domingo, 22/11, às 19h") || !strings.Contains(poll.text, "• Voz: 2 de 3 · ") ||
		!strings.Contains(poll.text, "estão livres") {
		t.Fatalf("question:\n%s", poll.text)
	}
	if strings.Join(poll.options, "|") != "✅ Segue assim|✏️ Vou ajustar no app" {
		t.Fatalf("options = %v", poll.options)
	}
}

func TestALeaderWhoCannotGoStillHearsAboutTheGap(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	var own int64
	for _, slot := range w.slotsOf(sunday, "Voz") {
		if slot.PersonName == "Líder" {
			own = slot.AssignmentID
		}
	}
	if _, err := w.bot.Decline(ctx, []int64{own}, true); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	polls := gapPolls(w.fake.to(w.phone("Líder")))
	if len(polls) != 1 || !strings.Contains(polls[0].text, "• Voz: 2 de 3 · Líder não pode; ") {
		t.Fatalf("the leader away should still be asked: %+v", polls)
	}
}

func TestARoleNobodyCouldFillIsLeftOut(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	drums := w.slotsOf(sunday, "Bateria")[0]
	if _, err := w.bot.Decline(ctx, []int64{drums.AssignmentID}, true); err != nil {
		t.Fatal(err)
	}
	w.fake.reset()

	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	if polls := gapPolls(w.fake.to(w.phone("Líder"))); len(polls) != 0 {
		t.Fatalf("only one person plays the drums, so there is nothing to ask: %+v", polls)
	}
}

func TestTheMinimumSilencesAShortRole(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	two := 2
	roles, _ := w.store.Ministry(ctx, w.ministry.ID)
	voice := roles.Roles[0]
	if err := w.store.UpdateRole(ctx, w.ministry.ID, voice.ID, store.RoleInput{Name: voice.Name, Slots: voice.Slots, Minimum: &two}); err != nil {
		t.Fatal(err)
	}
	w.emptyAVoice()

	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	if polls := gapPolls(w.fake.to(w.phone("Líder"))); len(polls) != 0 {
		t.Fatalf("two voices are enough: %+v", polls)
	}
}

func TestKeepingTheServiceAsItIsTellsTheOtherLeaders(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ministry, _ := w.store.Ministry(ctx, w.ministry.ID)
	for _, m := range ministry.Members {
		if m.PersonID == w.people["Edu"].ID {
			if err := w.store.PutMember(ctx, w.ministry.ID, m.PersonID, store.MemberInput{IsLeader: true, Roles: m.Roles}); err != nil {
				t.Fatal(err)
			}
		}
	}
	w.emptyAVoice()
	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	poll := gapPolls(w.fake.to(w.phone("Líder")))[0]
	if len(gapPolls(w.fake.to(w.phone("Edu")))) != 1 {
		t.Fatal("every leader should be asked")
	}
	w.fake.reset()

	w.vote(poll, "✅ Segue assim", false)
	w.vote(poll, "✅ Segue assim", false)
	if n := count(w.fake.to(w.phone("Líder")), "O culto de domingo, 22/11, às 19h segue assim"); n != 1 {
		t.Fatalf("leader got %d replies: %+v", n, w.fake.to(w.phone("Líder")))
	}
	if n := count(w.fake.to(w.phone("Edu")), "Líder decidiu que o culto de domingo, 22/11, às 19h segue assim"); n != 1 {
		t.Fatalf("the other leader got %d notes: %+v", n, w.fake.to(w.phone("Edu")))
	}
}
