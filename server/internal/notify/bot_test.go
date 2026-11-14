package notify_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"louvadeira/internal/messenger"
	"louvadeira/internal/notify"
	"louvadeira/internal/storage"
	"louvadeira/internal/store"
)

type message struct {
	id       string
	to       string
	text     string
	options  []string
	multiple bool
}

type fakeMessenger struct {
	sent []message
	next int
}

func (f *fakeMessenger) SendText(_ context.Context, to messenger.Address, text string) (messenger.MessageID, error) {
	f.next++
	id := fmt.Sprintf("msg-%d", f.next)
	f.sent = append(f.sent, message{id: id, to: string(to), text: text})
	return messenger.MessageID(id), nil
}

func (f *fakeMessenger) SendPoll(_ context.Context, to messenger.Address, poll messenger.Poll) (messenger.MessageID, error) {
	f.next++
	id := fmt.Sprintf("poll-%d", f.next)
	f.sent = append(f.sent, message{id: id, to: string(to), text: poll.Question, options: poll.Options, multiple: poll.Multiple})
	return messenger.MessageID(id), nil
}

func (f *fakeMessenger) to(phone string) []message {
	var out []message
	for _, m := range f.sent {
		if m.to == phone {
			out = append(out, m)
		}
	}
	return out
}

func (f *fakeMessenger) reset() { f.sent = nil }

var saoPaulo, _ = time.LoadLocation("America/Sao_Paulo")

var logOutput io.Writer = io.Discard

type world struct {
	t        *testing.T
	store    *store.Store
	fake     *fakeMessenger
	bot      *notify.Bot
	clock    time.Time
	ministry store.Ministry
	people   map[string]store.Person
	month    string
}

const group = "120363000000000000@g.us"

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, store: store.New(db), fake: &fakeMessenger{}, people: map[string]store.Person{}, month: "2026-11"}
	w.clock = time.Date(2026, 11, 13, 12, 0, 0, 0, saoPaulo)
	w.bot = &notify.Bot{
		Store:     w.store,
		Messenger: w.fake,
		Log:       slog.New(slog.NewTextHandler(logOutput, nil)),
		Location:  saoPaulo,
		Now:       func() time.Time { return w.clock },
		OwnPhone:  func() string { return "551188880001" },
	}

	s := w.store
	w.ministry, _ = s.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor", WhatsAppGroup: group})
	voice, _ := s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Voz", Slots: 3})
	guitar, _ := s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Violão", Slots: 1})
	bass, _ := s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	drums, _ := s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Bateria", Slots: 1})
	team := []struct {
		name   string
		leader bool
		roles  []store.MemberRole
	}{
		{"Líder", true, []store.MemberRole{{RoleID: voice.ID, Fixed: true}, {RoleID: guitar.ID, Fixed: true}}},
		{"Baterista", false, []store.MemberRole{{RoleID: drums.ID, Fixed: true}, {RoleID: guitar.ID}}},
		{"Ana", false, []store.MemberRole{{RoleID: voice.ID}}},
		{"Bia", false, []store.MemberRole{{RoleID: voice.ID}}},
		{"Cris", false, []store.MemberRole{{RoleID: voice.ID}}},
		{"Duda", false, []store.MemberRole{{RoleID: voice.ID}}},
		{"Edu", false, []store.MemberRole{{RoleID: bass.ID}}},
		{"Fred", false, []store.MemberRole{{RoleID: bass.ID}}},
	}
	for i, member := range team {
		p, err := s.CreatePerson(ctx, store.PersonInput{Name: member.name, Phone: fmt.Sprintf("+55 11 98888-%04d", i+1)})
		if err != nil {
			t.Fatal(err)
		}
		w.people[member.name] = p
		if err := s.PutMember(ctx, w.ministry.ID, p.ID, store.MemberInput{IsLeader: member.leader, Roles: member.roles}); err != nil {
			t.Fatal(err)
		}
	}
	third := 3
	thursday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Culto de quinta", Weekday: 4, StartTime: "19:30"})
	sunday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Culto de domingo", Weekday: 0, StartTime: "19:00", CommunionWeek: &third})
	if err := s.SetMinistryServices(ctx, w.ministry.ID, []int64{thursday.ID, sunday.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateDraft(ctx, w.ministry.ID, w.month, w.clock); err != nil {
		t.Fatal(err)
	}
	if err := s.PublishSchedule(ctx, w.ministry.ID, w.month, w.clock); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *world) phone(name string) string {
	return w.people[name].Phone
}

func (w *world) slotOf(date, role string) store.ScheduleSlot {
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
					return r.Slots[0]
				}
			}
		}
	}
	w.t.Fatalf("no %s slot on %s", role, date)
	return store.ScheduleSlot{}
}

func (w *world) vote(poll message, option string, fromMe bool) {
	w.t.Helper()
	w.tick(poll, fromMe, option)
}

/**
 * NOTE: tick sends the whole current selection of a poll, the way WhatsApp reports every change of a vote.
 */
func (w *world) tick(poll message, fromMe bool, options ...string) {
	w.t.Helper()
	var selected [][]byte
	for _, option := range options {
		found := false
		for _, label := range poll.options {
			found = found || label == option
		}
		if !found {
			w.t.Fatalf("poll %q has no option %q (options %v)", poll.text, option, poll.options)
		}
		selected = append(selected, messenger.OptionHash(option))
	}
	w.bot.HandleVote(context.Background(), messenger.Vote{PollID: messenger.MessageID(poll.id), Selected: selected, FromMe: fromMe})
}

func lastPoll(t *testing.T, messages []message) message {
	t.Helper()
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].options != nil {
			return messages[i]
		}
	}
	t.Fatal("no poll sent")
	return message{}
}

func containsText(messages []message, fragment string) bool {
	for _, m := range messages {
		if strings.Contains(m.text, fragment) {
			return true
		}
	}
	return false
}

func TestTickAsksEachPersonOncePerWeekWithTheWholeWeek(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())

	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	polls := w.fake.to(w.phone(bassist))
	var nextWeek message
	for _, p := range polls {
		if strings.Contains(p.text, "19/11") {
			nextWeek = p
		}
	}
	if !strings.Contains(nextWeek.text, "Domingo 22/11, 19h — Baixo") {
		t.Fatalf("next week poll should list Sunday too:\n%s", nextWeek.text)
	}
	want := []string{"✅ Confirmo", "❌ Não posso quinta 19/11", "❌ Não posso domingo 22/11", "❌ Não posso em nenhum"}
	if strings.Join(nextWeek.options, "|") != strings.Join(want, "|") {
		t.Fatalf("options = %v", nextWeek.options)
	}
	if len(w.fake.sent) != 10 {
		t.Fatalf("sent %d polls, want 5 people x 2 weeks", len(w.fake.sent))
	}

	w.fake.reset()
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 0 {
		t.Fatalf("second tick sent %d messages", len(w.fake.sent))
	}
}

func TestQuietHoursSendNothing(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 13, 22, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 0 {
		t.Fatalf("sent %d messages at night", len(w.fake.sent))
	}
}

func TestDeclineInvitesTheOtherBassistAndTheFirstAcceptanceTakesTheSlot(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	absent := w.slotOf("2026-11-15", "Baixo").PersonName
	substitute := map[string]string{"Edu": "Fred", "Fred": "Edu"}[absent]

	poll := w.fake.sentPollFor(t, w.phone(absent), "15/11")
	w.fake.reset()
	w.vote(poll, "❌ Não posso", false)

	invite := lastPoll(t, w.fake.to(w.phone(substitute)))
	if !strings.Contains(invite.text, absent+" não poderá servir no Louvor em domingo 15/11, 19h (Baixo)") {
		t.Fatalf("invite = %q", invite.text)
	}
	leader := w.fake.to(w.phone("Líder"))
	if !containsText(leader, "Aviso com menos de 72h") || !containsText(leader, "Convidei para cobrir: "+substitute) {
		t.Fatalf("leader messages = %+v", leader)
	}
	if !containsText(w.fake.to(w.phone(absent)), "chamei "+substitute) {
		t.Fatalf("absent reply = %+v", w.fake.to(w.phone(absent)))
	}
	if slot := w.slotOf("2026-11-15", "Baixo"); slot.Status != store.AssignmentDeclined || !slot.CoverOpen {
		t.Fatalf("slot after decline = %+v", slot)
	}

	w.fake.reset()
	w.vote(invite, "✅ Posso cobrir", false)
	slot := w.slotOf("2026-11-15", "Baixo")
	if slot.PersonName != substitute || slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
		t.Fatalf("slot after cover = %+v", slot)
	}
	if !containsText(w.fake.to(group), "🔄 Troca no Louvor: "+substitute+" cobre "+absent) {
		t.Fatalf("group did not get the swap: %+v", w.fake.to(group))
	}
	if !containsText(w.fake.to(w.phone(absent)), substitute+" vai cobrir você") {
		t.Fatalf("absent was not told: %+v", w.fake.to(w.phone(absent)))
	}

	w.fake.reset()
	w.vote(invite, "✅ Posso cobrir", false)
	if len(w.fake.sent) != 0 {
		t.Fatalf("voting again for a slot already taken sent %+v", w.fake.sent)
	}
}

func TestWhenEveryoneRefusesTheLeaderIsAlerted(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	thursday := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[thursday]

	poll := w.fake.sentPollFor(t, w.phone(thursday), "19/11")
	w.fake.reset()
	w.vote(poll, "❌ Não posso quinta 19/11", false)
	if containsText(w.fake.to(w.phone("Líder")), "menos de 72h") {
		t.Fatal("six days ahead is not a late notice")
	}
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	w.fake.reset()
	w.vote(invite, "❌ Não posso", false)
	if !containsText(w.fake.to(w.phone("Líder")), "Ninguém pôde cobrir "+thursday) {
		t.Fatalf("leader messages = %+v", w.fake.to(w.phone("Líder")))
	}
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.CoverOpen {
		t.Fatalf("request still open: %+v", slot)
	}
}

func TestConfirmVoteConfirmsTheWholeWeek(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "✅ Confirmo", false)
	for _, date := range []string{"2026-11-19", "2026-11-22"} {
		if slot := w.slotOf(date, "Baixo"); slot.Status != store.AssignmentConfirmed {
			t.Fatalf("%s slot = %+v", date, slot)
		}
	}
	if !containsText(w.fake.to(w.phone(bassist)), "Presença confirmada") {
		t.Fatal("no thank-you message")
	}
}

func TestVotesFromTheLinkedPhoneOnlyCountForItsOwner(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName

	w.vote(w.fake.sentPollFor(t, w.phone(bassist), "19/11"), "✅ Confirmo", true)
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentScheduled {
		t.Fatalf("leader's own phone answered for a member: %+v", slot)
	}
	w.vote(w.fake.sentPollFor(t, w.phone("Líder"), "19/11"), "✅ Confirmo", true)
	if slot := w.slotOf("2026-11-19", "Violão"); slot.Status != store.AssignmentConfirmed {
		t.Fatalf("leader's own vote did not count: %+v", slot)
	}
}

func TestTestModeSkipsBlockedPeopleWithoutForgettingThem(t *testing.T) {
	w := newWorld(t)
	w.bot.Messenger = messenger.WithAllowlist(w.fake, []messenger.Address{messenger.Address(w.phone("Líder"))})
	w.bot.Tick(context.Background())
	for _, m := range w.fake.sent {
		if m.to != w.phone("Líder") {
			t.Fatalf("test mode reached %s", m.to)
		}
	}
	if len(w.fake.sent) != 2 {
		t.Fatalf("leader got %d polls, want 2", len(w.fake.sent))
	}

	w.fake.reset()
	w.bot.Messenger = w.fake
	w.bot.Tick(context.Background())
	if len(w.fake.sent) != 8 {
		t.Fatalf("after going live %d polls were sent, want the 8 skipped before", len(w.fake.sent))
	}
}

func TestRemindersAndPendingAlerts(t *testing.T) {
	w := newWorld(t)
	w.clock = time.Date(2026, 11, 13, 10, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	if containsText(w.fake.to(w.phone("Líder")), "⏳") {
		t.Fatal("pending alert right after asking")
	}
	w.clock = time.Date(2026, 11, 14, 11, 0, 0, 0, saoPaulo)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-15", "Baixo").PersonName
	if !containsText(w.fake.to(w.phone(bassist)), "Amanhã (domingo 15/11) tem culto às 19h") ||
		!containsText(w.fake.to(w.phone(bassist)), "Santa Ceia") {
		t.Fatalf("reminder = %+v", w.fake.to(w.phone(bassist)))
	}
	if !containsText(w.fake.to(w.phone("Líder")), "⏳ Ainda não confirmaram para domingo 15/11, 19h") {
		t.Fatalf("leader = %+v", w.fake.to(w.phone("Líder")))
	}
}

func TestSummaryUsesTheTeamFormat(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	slot := w.slotOf("2026-11-19", "Baixo")
	if err := w.store.SetAssignment(ctx, w.ministry.ID, w.month, slot.AssignmentID, nil, w.clock); err != nil {
		t.Fatal(err)
	}
	ministry, _ := w.store.Ministry(ctx, w.ministry.ID)
	view, _ := w.store.ScheduleView(ctx, w.ministry.ID, w.month)
	summary := notify.Summary(ministry, view)

	if !strings.HasPrefix(summary, "Paz, pessoal! 🎶 Escala do *Louvor* de novembro:") {
		t.Fatalf("header:\n%s", summary)
	}
	if !strings.Contains(summary, "*Semana de 19/11/2026*") || !strings.Contains(summary, "⚠️ falta Baixo") {
		t.Fatalf("week or missing role:\n%s", summary)
	}
	if strings.Contains(summary, "Líder") || strings.Contains(summary, "Baterista") {
		t.Fatalf("fixed members should be implied:\n%s", summary)
	}
	if !strings.Contains(summary, "(15/11/2026)") || !strings.Contains(summary, "🍷") {
		t.Fatalf("communion Sunday missing:\n%s", summary)
	}
}

func (f *fakeMessenger) sentPollFor(t *testing.T, phone, fragment string) message {
	t.Helper()
	for _, m := range f.to(phone) {
		if m.options != nil && strings.Contains(m.text, fragment) {
			return m
		}
	}
	t.Fatalf("no poll to %s mentioning %s", phone, fragment)
	return message{}
}

func TestChangingTheVoteBackToConfirmCancelsTheSwap(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")

	w.vote(poll, "❌ Não posso quinta 19/11", false)
	invite := lastPoll(t, w.fake.to(w.phone(other)))
	w.fake.reset()
	w.vote(poll, "✅ Confirmo", false)

	if slot := w.slotOf("2026-11-19", "Baixo"); slot.PersonName != bassist || slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
		t.Fatalf("slot after changing back = %+v", slot)
	}
	if !containsText(w.fake.to(w.phone(other)), "Não precisa mais cobrir") {
		t.Fatalf("invitee not released: %+v", w.fake.to(w.phone(other)))
	}
	if !containsText(w.fake.to(w.phone("Líder")), "voltou atrás") {
		t.Fatalf("leader not told: %+v", w.fake.to(w.phone("Líder")))
	}

	w.fake.reset()
	w.vote(invite, "✅ Posso cobrir", false)
	if !containsText(w.fake.to(w.phone(other)), "já foi resolvida") {
		t.Fatalf("late acceptance = %+v", w.fake.sent)
	}
}

func TestChangingBackAfterSomeoneCoveredKeepsTheSubstitute(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	other := map[string]string{"Edu": "Fred", "Fred": "Edu"}[bassist]
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")

	w.vote(poll, "❌ Não posso quinta 19/11", false)
	w.vote(lastPoll(t, w.fake.to(w.phone(other))), "✅ Posso cobrir", false)
	w.fake.reset()
	w.vote(poll, "✅ Confirmo", false)

	if slot := w.slotOf("2026-11-19", "Baixo"); slot.PersonName != other {
		t.Fatalf("substitute lost the slot: %+v", slot)
	}
	if !containsText(w.fake.to(w.phone(bassist)), "já foi coberta por "+other) {
		t.Fatalf("bassist reply = %+v", w.fake.to(w.phone(bassist)))
	}
}

func TestRemovingTheVoteChangesNothing(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.vote(poll, "✅ Confirmo", false)
	w.fake.reset()
	w.bot.HandleVote(context.Background(), messenger.Vote{PollID: messenger.MessageID(poll.id)})
	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentConfirmed || len(w.fake.sent) != 0 {
		t.Fatalf("slot = %+v, sent = %+v", slot, w.fake.sent)
	}
}

func TestQuickChangesOnlyCountTheLastVote(t *testing.T) {
	w := newWorld(t)
	w.bot.Tick(context.Background())
	bassist := w.slotOf("2026-11-19", "Baixo").PersonName
	poll := w.fake.sentPollFor(t, w.phone(bassist), "19/11")
	w.bot.Settle = 50 * time.Millisecond
	w.fake.reset()

	w.vote(poll, "❌ Não posso quinta 19/11", false)
	w.vote(poll, "✅ Confirmo", false)
	time.Sleep(200 * time.Millisecond)

	if slot := w.slotOf("2026-11-19", "Baixo"); slot.Status != store.AssignmentConfirmed || slot.CoverOpen {
		t.Fatalf("slot = %+v", slot)
	}
	if containsText(w.fake.sent, "não pode servir") {
		t.Fatalf("the quick decline still went out: %+v", w.fake.sent)
	}
}
