package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"louvadeira/internal/store"
)

var generatedAt = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

type worship struct {
	ministry store.Ministry
	voice    store.Role
	bass     store.Role
	people   map[string]store.Person
}

func setUpWorship(t *testing.T, s *store.Store) worship {
	t.Helper()
	ctx := context.Background()
	w := worship{people: map[string]store.Person{}}
	w.ministry, _ = s.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	w.voice, _ = s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Voz", Slots: 2})
	w.bass, _ = s.CreateRole(ctx, w.ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	for _, name := range []string{"Ana", "Bia", "Cris", "Duda", "Eli"} {
		p, err := s.CreatePerson(ctx, store.PersonInput{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		w.people[name] = p
		role := w.voice.ID
		if name == "Eli" {
			role = w.bass.ID
		}
		if err := s.PutMember(ctx, w.ministry.ID, p.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: role}}}); err != nil {
			t.Fatal(err)
		}
	}
	thursday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Quinta", Weekday: 4, StartTime: "19:30"})
	sunday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Domingo", Weekday: 0, StartTime: "19:00"})
	if err := s.SetMinistryServices(ctx, w.ministry.ID, []int64{thursday.ID, sunday.ID}); err != nil {
		t.Fatal(err)
	}
	return w
}

func voicesOf(view store.ScheduleView, week int) []int64 {
	var ids []int64
	for _, role := range view.Weeks[week].Services[0].Roles {
		if role.Name == "Voz" {
			for _, slot := range role.Slots {
				ids = append(ids, slot.PersonID)
			}
		}
	}
	return ids
}

func TestGenerateDraftNeedsServicesAndRoles(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	m, _ := s.CreateMinistry(ctx, store.MinistryInput{Name: "Mídia"})
	if err := s.GenerateDraft(ctx, m.ID, "2026-12", generatedAt); !isValidation(err) {
		t.Fatalf("no services: err = %v", err)
	}
	if err := s.GenerateDraft(ctx, 999, "2026-12", generatedAt); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing ministry: err = %v", err)
	}
}

func TestGenerateDraftBuildsTheMonthWithWeeksServicesAndSlots(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	if err := s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt); err != nil {
		t.Fatal(err)
	}
	view, err := s.ScheduleView(ctx, w.ministry.ID, "2026-12")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != store.StatusDraft || len(view.Weeks) != 5 || view.Warnings != 0 {
		t.Fatalf("view: status %s, %d weeks, %d warnings", view.Status, len(view.Weeks), view.Warnings)
	}
	first := view.Weeks[0]
	if first.Start != "2026-11-30" || len(first.Services) != 2 || first.Services[0].Date != "2026-12-03" || first.Services[1].Name != "Domingo" {
		t.Fatalf("first week = %+v", first)
	}
	roles := first.Services[0].Roles
	if len(roles) != 2 || roles[0].Name != "Voz" || len(roles[0].Slots) != 2 || roles[1].Slots[0].PersonName != "Eli" {
		t.Fatalf("roles = %+v", roles)
	}
}

func TestPublishedSchedulesCannotBeRegeneratedAndFeedTheNextMonth(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	_ = s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt)
	if err := s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt); err != nil {
		t.Fatalf("regenerating a draft: %v", err)
	}
	if err := s.PublishSchedule(ctx, w.ministry.ID, "2026-12", generatedAt); err != nil {
		t.Fatal(err)
	}
	if err := s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt); !errors.Is(err, store.ErrPublished) {
		t.Fatalf("regenerating a published schedule: err = %v", err)
	}
	if err := s.PublishSchedule(ctx, w.ministry.ID, "2026-12", generatedAt); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("publishing twice: err = %v", err)
	}

	december, _ := s.ScheduleView(ctx, w.ministry.ID, "2026-12")
	lastPair := voicesOf(december, len(december.Weeks)-1)
	if err := s.GenerateDraft(ctx, w.ministry.ID, "2027-01", generatedAt); err != nil {
		t.Fatal(err)
	}
	january, _ := s.ScheduleView(ctx, w.ministry.ID, "2027-01")
	firstPair := voicesOf(january, 0)
	if sameSet(lastPair, firstPair) {
		t.Fatalf("January starts with December's last pair %v", firstPair)
	}
}

func TestEditingSlotsAndWarnings(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	_ = s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt)
	view, _ := s.ScheduleView(ctx, w.ministry.ID, "2026-12")
	bassSlot := view.Weeks[0].Services[0].Roles[1].Slots[0]

	outsider, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Fora"})
	if err := s.SetAssignment(ctx, w.ministry.ID, "2026-12", bassSlot.AssignmentID, &outsider.ID, generatedAt); !isValidation(err) {
		t.Fatalf("non-member: err = %v", err)
	}
	if err := s.SetAssignment(ctx, w.ministry.ID, "2026-11", bassSlot.AssignmentID, nil, generatedAt); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong month: err = %v", err)
	}
	if err := s.SetAssignment(ctx, w.ministry.ID, "2026-12", bassSlot.AssignmentID, nil, generatedAt); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: w.people["Eli"].ID, StartDate: "2026-12-06", Reason: store.ReasonWork})
	if err != nil {
		t.Fatal(err)
	}

	view, _ = s.ScheduleView(ctx, w.ministry.ID, "2026-12")
	thursday, sunday := view.Weeks[0].Services[0], view.Weeks[0].Services[1]
	if thursday.Roles[1].Slots[0].Warning != store.WarningEmpty {
		t.Fatalf("cleared slot warning = %q", thursday.Roles[1].Slots[0].Warning)
	}
	if sunday.Roles[1].Slots[0].Warning != store.WarningUnavailable {
		t.Fatalf("unavailable slot warning = %q", sunday.Roles[1].Slots[0].Warning)
	}
	if view.Warnings != 2 {
		t.Fatalf("warnings = %d, want 2", view.Warnings)
	}
}

func TestUnavailabilities(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Gabi"})
	if _, err := s.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: p.ID, StartDate: "2026-12-10", EndDate: "2026-12-01", Reason: store.ReasonTravel}); !isValidation(err) {
		t.Fatalf("end before start: err = %v", err)
	}
	if _, err := s.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: p.ID, StartDate: "2026-12-10", Reason: "passeio"}); !isValidation(err) {
		t.Fatalf("reason outside the list: err = %v", err)
	}
	if _, err := s.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: 999, StartDate: "2026-12-10", Reason: store.ReasonTravel}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing person: err = %v", err)
	}
	u, err := s.CreateUnavailability(ctx, store.UnavailabilityInput{PersonID: p.ID, StartDate: "2026-12-10", EndDate: "2026-12-20", Reason: store.ReasonTravel})
	if err != nil || u.PersonName != "Gabi" || u.Reason != store.ReasonTravel {
		t.Fatalf("created = %+v, err = %v", u, err)
	}
	for _, c := range []struct {
		from, to string
		want     int
	}{{"2026-12-01", "2026-12-09", 0}, {"2026-12-15", "2026-12-15", 1}, {"2026-12-20", "2026-12-31", 1}} {
		list, _ := s.ListUnavailabilities(ctx, c.from, c.to)
		if len(list) != c.want {
			t.Errorf("list %s..%s = %d, want %d", c.from, c.to, len(list), c.want)
		}
	}
	if err := s.DeleteUnavailability(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
}

func TestReorderRoles(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	if err := s.ReorderRoles(ctx, w.ministry.ID, []int64{w.bass.ID}); !isValidation(err) {
		t.Fatalf("missing role: err = %v", err)
	}
	if err := s.ReorderRoles(ctx, w.ministry.ID, []int64{w.bass.ID, w.bass.ID}); !isValidation(err) {
		t.Fatalf("duplicate role: err = %v", err)
	}
	if err := s.ReorderRoles(ctx, w.ministry.ID, []int64{w.bass.ID, w.voice.ID}); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Ministry(ctx, w.ministry.ID)
	if d.Roles[0].ID != w.bass.ID {
		t.Fatalf("roles = %+v, want bass first", d.Roles)
	}
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int64]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestCommunionSundaysAreMarked(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	services, _ := s.ListServices(ctx)
	third := 3
	for _, sv := range services {
		if sv.Weekday == 0 {
			updated, err := s.UpdateService(ctx, sv.ID, store.ServiceInput{Name: sv.Name, Weekday: 0, StartTime: sv.StartTime, Active: true, CommunionWeek: &third})
			if err != nil || updated.CommunionWeek == nil || *updated.CommunionWeek != 3 {
				t.Fatalf("updated = %+v, err = %v", updated, err)
			}
		}
	}
	_ = s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt)
	view, _ := s.ScheduleView(ctx, w.ministry.ID, "2026-12")
	var marked []string
	for _, week := range view.Weeks {
		for _, sv := range week.Services {
			if sv.Communion {
				marked = append(marked, sv.Date)
			}
		}
	}
	if len(marked) != 1 || marked[0] != "2026-12-20" {
		t.Fatalf("communion dates = %v, want the third Sunday (2026-12-20)", marked)
	}
	five := 5
	if _, err := s.CreateService(ctx, store.ServiceInput{Name: "Extra", Weekday: 0, StartTime: "08:00", CommunionWeek: &five}); !isValidation(err) {
		t.Fatalf("communion week 5: err = %v", err)
	}
}

func TestDiscardDraftOnlyForDrafts(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	w := setUpWorship(t, s)
	if err := s.DiscardDraft(ctx, w.ministry.ID, "2026-12"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("nothing to discard: err = %v", err)
	}
	_ = s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt)
	if err := s.DiscardDraft(ctx, w.ministry.ID, "2026-12"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScheduleView(ctx, w.ministry.ID, "2026-12"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("after discard: err = %v", err)
	}
	_ = s.GenerateDraft(ctx, w.ministry.ID, "2026-12", generatedAt)
	_ = s.PublishSchedule(ctx, w.ministry.ID, "2026-12", generatedAt)
	if err := s.DiscardDraft(ctx, w.ministry.ID, "2026-12"); !errors.Is(err, store.ErrPublished) {
		t.Fatalf("discard published: err = %v", err)
	}
}
