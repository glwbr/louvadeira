package notify_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"louvadeira/internal/messenger"
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
	if strings.Join(poll.options, "|") != "✅ Segue assim|📣 Pedir no grupo|✏️ Vou ajustar no app" {
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

func (w *world) voteFrom(poll message, name, option string) {
	w.t.Helper()
	w.bot.HandleVote(context.Background(), messenger.Vote{PollID: messenger.MessageID(poll.id),
		Selected: [][]byte{messenger.OptionHash(option)}, From: messenger.Address(w.phone(name))})
}

func (w *world) freeVoices() []string {
	var serving []string
	for _, slot := range w.slotsOf(sunday, "Voz") {
		serving = append(serving, slot.PersonName)
	}
	var free []string
	for _, name := range []string{"Ana", "Bia", "Cris", "Duda"} {
		if !slices.Contains(serving, name) {
			free = append(free, name)
		}
	}
	return free
}

func (w *world) askTheGroup() message {
	w.t.Helper()
	w.tickAt(time.Date(2026, 11, 19, 9, 0, 0, 0, saoPaulo))
	leader := gapPolls(w.fake.to(w.phone("Líder")))[0]
	w.fake.reset()
	w.vote(leader, "📣 Pedir no grupo", false)
	if !containsText(w.fake.to(w.phone("Líder")), "Pedi no grupo do Louvor") {
		w.t.Fatalf("leader messages = %+v", w.fake.to(w.phone("Líder")))
	}
	call := lastPoll(w.t, w.fake.to(group))
	w.fake.reset()
	return call
}

func (w *world) voiceOf(name string) store.ScheduleSlot {
	for _, slot := range w.slotsOf(sunday, "Voz") {
		if slot.PersonName == name {
			return slot
		}
	}
	return store.ScheduleSlot{}
}

func TestAskingTheGroupGivesThePlaceToTheFirstVolunteer(t *testing.T) {
	w := newWorld(t)
	w.emptyAVoice()
	call := w.askTheGroup()
	if !strings.Contains(call.text, "Ainda falta gente nesse culto: Voz.") ||
		strings.Join(call.options, "|") != "🙋 Eu posso (Voz)|😕 Dessa vez não" {
		t.Fatalf("group poll = %q %v", call.text, call.options)
	}

	free := w.freeVoices()
	first, second := free[0], free[1]
	w.voteFrom(call, first, "🙋 Eu posso (Voz)")
	w.voteFrom(call, second, "🙋 Eu posso (Voz)")
	if slot := w.voiceOf(first); slot.Status != store.AssignmentConfirmed {
		t.Fatalf("%s should be in the schedule, confirmed: %+v", first, slot)
	}
	if !containsText(w.fake.to(group), "@"+w.phone(first)+" entra na escala") {
		t.Fatalf("group messages = %+v", w.fake.to(group))
	}
	if !containsText(w.fake.to(w.phone(first)), "Combinado") || !containsText(w.fake.to(w.phone(second)), "já foi resolvido") {
		t.Fatalf("first = %+v, second = %+v", w.fake.to(w.phone(first)), w.fake.to(w.phone(second)))
	}
}

func TestTheGroupCallChecksWhoVolunteers(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.emptyAVoice()
	call := w.askTheGroup()

	w.voteFrom(call, "Baterista", "🙋 Eu posso (Voz)")
	if !containsText(w.fake.to(w.phone("Baterista")), "Você já está nesse culto") {
		t.Fatalf("someone already serving = %+v", w.fake.to(w.phone("Baterista")))
	}

	outsider, err := w.store.CreatePerson(ctx, store.PersonInput{Name: "Gil", Phone: "+55 11 98888-0099"})
	if err != nil {
		t.Fatal(err)
	}
	w.people["Gil"] = outsider
	w.voteFrom(call, "Gil", "🙋 Eu posso (Voz)")
	if !containsText(w.fake.to(w.phone("Líder")), "Gil* se ofereceu para Voz no culto de domingo, 22/11, às 19h, mas não está na equipe do Louvor") {
		t.Fatalf("leader messages = %+v", w.fake.to(w.phone("Líder")))
	}

	sent := len(w.fake.sent)
	w.bot.HandleVote(ctx, messenger.Vote{PollID: messenger.MessageID(call.id), Selected: [][]byte{messenger.OptionHash("🙋 Eu posso (Voz)")},
		From: "5511977770000"})
	w.voteFrom(call, w.freeVoices()[0], "😕 Dessa vez não")
	if len(w.fake.sent) != sent {
		t.Fatalf("a stranger or a no should get no answer: %+v", w.fake.sent[sent:])
	}
}

func TestVolunteersVotingAtOnceAreEachHeard(t *testing.T) {
	w := newWorld(t)
	w.emptyAVoice()
	call := w.askTheGroup()
	w.bot.Settle = 50 * time.Millisecond

	free := w.freeVoices()
	w.voteFrom(call, free[0], "🙋 Eu posso (Voz)")
	w.voteFrom(call, free[1], "🙋 Eu posso (Voz)")
	time.Sleep(300 * time.Millisecond)

	replies := 0
	for _, name := range free[:2] {
		if containsText(w.fake.to(w.phone(name)), "Combinado") || containsText(w.fake.to(w.phone(name)), "já foi resolvido") {
			replies++
		}
	}
	if replies != 2 {
		t.Fatalf("both volunteers should get an answer: %+v", w.fake.sent)
	}
}

func TestAVolunteerTakesADeclinedPlaceAndClosesTheSwap(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	own := w.voiceOf("Líder")
	if _, err := w.bot.Decline(ctx, []int64{own.AssignmentID}, true); err != nil {
		t.Fatal(err)
	}
	call := w.askTheGroup()

	free := w.freeVoices()
	volunteer := free[0]
	w.voteFrom(call, volunteer, "🙋 Eu posso (Voz)")
	if slot := w.voiceOf(volunteer); slot.AssignmentID != own.AssignmentID || slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
		t.Fatalf("%s should hold the leader's place: %+v", volunteer, slot)
	}
	if !containsText(w.fake.to(group), "@"+w.phone(volunteer)+" substitui @"+w.phone("Líder")) {
		t.Fatalf("group messages = %+v", w.fake.to(group))
	}
	if !containsText(w.fake.to(w.phone("Líder")), "vai ficar no seu lugar") {
		t.Fatalf("leader messages = %+v", w.fake.to(w.phone("Líder")))
	}
	if containsText(w.fake.to(w.phone(volunteer)), "já vai substituir") {
		t.Fatal("the volunteer was told someone else took the place")
	}
	if len(free) > 1 && !containsText(w.fake.to(w.phone(free[1])), volunteer+" já vai substituir") {
		t.Fatalf("the other invited voice = %+v", w.fake.to(w.phone(free[1])))
	}
}
