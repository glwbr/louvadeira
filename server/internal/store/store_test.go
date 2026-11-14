package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"louvadeira/internal/storage"
	"louvadeira/internal/store"
)

func newStore(t *testing.T) *store.Store {
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
	return store.New(db)
}

func isValidation(err error) bool {
	var v store.ValidationError
	return errors.As(err, &v)
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"(11) 98888-7777":   "5511988887777",
		"+55 11 98888-7777": "5511988887777",
		"11 3333-4444":      "551133334444",
		"+1 (415) 555-0100": "14155550100",
		"":                  "",
	}
	for in, want := range cases {
		got, err := store.NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := store.NormalizePhone("12345"); !isValidation(err) {
		t.Errorf("short number: err = %v, want validation error", err)
	}
}

func TestPeopleRejectDuplicatePhonesInAnyFormat(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if _, err := s.CreatePerson(ctx, store.PersonInput{Name: "  Ana   Souza ", Phone: "(11) 98888-7777"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreatePerson(ctx, store.PersonInput{Name: "Outra", Phone: "+55 11 98888-7777"})
	if !isValidation(err) {
		t.Fatalf("duplicate phone: err = %v, want validation error", err)
	}
	people, err := s.ListPeople(ctx)
	if err != nil || len(people) != 1 || people[0].Name != "Ana Souza" || people[0].Phone != "5511988887777" {
		t.Fatalf("people = %+v, err = %v", people, err)
	}
}

func TestArchivingAPersonRemovesThemFromMinistriesAndFreesThePhone(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	person, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Bruno", Phone: "11988887777"})
	ministry, _ := s.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	if err := s.PutMember(ctx, ministry.ID, person.ID, store.MemberInput{}); err != nil {
		t.Fatal(err)
	}

	if err := s.ArchivePerson(ctx, person.ID); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Ministry(ctx, ministry.ID)
	if len(detail.Members) != 0 {
		t.Fatalf("archived person still a member: %+v", detail.Members)
	}
	if _, err := s.CreatePerson(ctx, store.PersonInput{Name: "Bruno novo", Phone: "11988887777"}); err != nil {
		t.Fatalf("phone not freed: %v", err)
	}
	if err := s.ArchivePerson(ctx, person.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second archive: err = %v, want ErrNotFound", err)
	}
}

func TestMinistryDetailGathersRolesMembersAndServices(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	ministry, err := s.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	if err != nil {
		t.Fatal(err)
	}
	voice, _ := s.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Voz", Slots: 3})
	guitar, _ := s.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Violão", Slots: 1})
	leader, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Carlos"})
	singer, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Bia"})
	thursday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Culto de quinta", Weekday: 4, StartTime: "19:30"})
	sunday, _ := s.CreateService(ctx, store.ServiceInput{Name: "Culto de domingo à noite", Weekday: 0, StartTime: "19:00"})

	err = s.PutMember(ctx, ministry.ID, leader.ID, store.MemberInput{IsLeader: true, Roles: []store.MemberRole{
		{RoleID: guitar.ID, Fixed: true}, {RoleID: voice.ID, Fixed: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutMember(ctx, ministry.ID, singer.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: voice.ID}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMinistryServices(ctx, ministry.ID, []int64{thursday.ID, sunday.ID}); err != nil {
		t.Fatal(err)
	}

	d, err := s.Ministry(ctx, ministry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.RotationUnit != store.RotationWeek || len(d.Roles) != 2 || d.Roles[0].Name != "Voz" || d.Roles[0].Slots != 3 {
		t.Fatalf("ministry/roles = %+v", d)
	}
	if len(d.Members) != 2 || d.Members[0].Name != "Bia" || d.Members[1].Name != "Carlos" {
		t.Fatalf("members = %+v", d.Members)
	}
	carlos := d.Members[1]
	if !carlos.IsLeader || len(carlos.Roles) != 2 || carlos.Roles[0].RoleID != voice.ID || !carlos.Roles[0].Fixed {
		t.Fatalf("leader = %+v", carlos)
	}
	if len(d.ServiceIDs) != 2 || d.ServiceIDs[0] != sunday.ID {
		t.Fatalf("services = %v, want Sunday first", d.ServiceIDs)
	}

	if err := s.PutMember(ctx, ministry.ID, singer.ID, store.MemberInput{}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Ministry(ctx, ministry.ID)
	if len(d.Members[0].Roles) != 0 {
		t.Fatalf("roles were not replaced: %+v", d.Members[0])
	}
}

func TestMembersCannotTakeRolesFromAnotherMinistry(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	worship, _ := s.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	media, _ := s.CreateMinistry(ctx, store.MinistryInput{Name: "Mídia"})
	camera, _ := s.CreateRole(ctx, media.ID, store.RoleInput{Name: "Câmera", Slots: 1})
	person, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Duda"})

	err := s.PutMember(ctx, worship.ID, person.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: camera.ID}}})
	if !isValidation(err) {
		t.Fatalf("err = %v, want validation error", err)
	}
	d, _ := s.Ministry(ctx, worship.ID)
	if len(d.Members) != 0 {
		t.Fatalf("failed put left a member behind: %+v", d.Members)
	}
}

func TestMinistryAndRoleValidation(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if _, err := s.CreateMinistry(ctx, store.MinistryInput{Name: " "}); !isValidation(err) {
		t.Errorf("blank name: err = %v", err)
	}
	if _, err := s.CreateMinistry(ctx, store.MinistryInput{Name: "Dança", RotationUnit: "mensal"}); !isValidation(err) {
		t.Errorf("bad rotation: err = %v", err)
	}
	m, _ := s.CreateMinistry(ctx, store.MinistryInput{Name: "Dança", RotationUnit: store.RotationService})
	if _, err := s.CreateMinistry(ctx, store.MinistryInput{Name: "Dança"}); !isValidation(err) {
		t.Errorf("duplicate ministry: err = %v", err)
	}
	if _, err := s.CreateRole(ctx, m.ID, store.RoleInput{Name: "Solo", Slots: 0}); !isValidation(err) {
		t.Errorf("zero slots: err = %v", err)
	}
	if _, err := s.CreateRole(ctx, 999, store.RoleInput{Name: "Solo", Slots: 1}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing ministry: err = %v", err)
	}
	if err := s.ArchiveMinistry(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMinistry(ctx, store.MinistryInput{Name: "Dança"}); err != nil {
		t.Errorf("name not reusable after archive: %v", err)
	}
}

func TestServiceValidationAndUpdate(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	if _, err := s.CreateService(ctx, store.ServiceInput{Name: "Culto", Weekday: 7, StartTime: "19:30"}); !isValidation(err) {
		t.Errorf("bad weekday: err = %v", err)
	}
	if _, err := s.CreateService(ctx, store.ServiceInput{Name: "Culto", Weekday: 2, StartTime: "25:00"}); !isValidation(err) {
		t.Errorf("bad time: err = %v", err)
	}
	sv, err := s.CreateService(ctx, store.ServiceInput{Name: "Culto de terça", Weekday: 2, StartTime: "7:30"})
	if err != nil || !sv.Active || sv.StartTime != "07:30" {
		t.Fatalf("service = %+v, err = %v", sv, err)
	}
	sv, err = s.UpdateService(ctx, sv.ID, store.ServiceInput{Name: "Culto de terça", Weekday: 2, StartTime: "19:30", Active: false})
	if err != nil || sv.Active || sv.StartTime != "19:30" {
		t.Fatalf("updated = %+v, err = %v", sv, err)
	}
}

func TestLoginTokenWorksOnceAndExpires(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	admin, err := s.EnsureAdmin(ctx, store.PersonInput{Name: "Admin", Phone: "11988887777"})
	if err != nil || !admin.IsAdmin {
		t.Fatalf("admin = %+v, err = %v", admin, err)
	}
	now := time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)

	token, err := s.CreateLoginToken(ctx, admin.ID, now, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	session, person, err := s.RedeemLoginToken(ctx, token, now.Add(time.Minute), 30*24*time.Hour)
	if err != nil || person.ID != admin.ID {
		t.Fatalf("redeem: person = %+v, err = %v", person, err)
	}
	if _, _, err := s.RedeemLoginToken(ctx, token, now.Add(2*time.Minute), time.Hour); !errors.Is(err, store.ErrInvalidToken) {
		t.Fatalf("second redeem: err = %v, want ErrInvalidToken", err)
	}

	if p, err := s.SessionPerson(ctx, session, now.Add(time.Hour)); err != nil || p.ID != admin.ID {
		t.Fatalf("session: person = %+v, err = %v", p, err)
	}
	if _, err := s.SessionPerson(ctx, session, now.Add(31*24*time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired session: err = %v", err)
	}

	late, _ := s.CreateLoginToken(ctx, admin.ID, now, 15*time.Minute)
	if _, _, err := s.RedeemLoginToken(ctx, late, now.Add(16*time.Minute), time.Hour); !errors.Is(err, store.ErrInvalidToken) {
		t.Fatalf("expired token: err = %v", err)
	}

	if err := s.DeleteSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionPerson(ctx, session, now.Add(time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted session: err = %v", err)
	}
}

func TestEnsureAdminPromotesAnExistingPerson(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	p, _ := s.CreatePerson(ctx, store.PersonInput{Name: "Eli", Phone: "11977776666"})
	admin, err := s.EnsureAdmin(ctx, store.PersonInput{Name: "Outro nome", Phone: "+55 11 97777-6666"})
	if err != nil || admin.ID != p.ID || !admin.IsAdmin || admin.Name != "Eli" {
		t.Fatalf("admin = %+v, err = %v", admin, err)
	}
}

func TestCommunionIsOneServiceAMonth(t *testing.T) {
	s, ctx := newStore(t), context.Background()
	morning, _ := s.CreateService(ctx, store.ServiceInput{Name: "Domingo de manhã", Weekday: 0, StartTime: "09:00"})
	evening, _ := s.CreateService(ctx, store.ServiceInput{Name: "Domingo à noite", Weekday: 0, StartTime: "19:00"})
	weeks := func() (*int, *int) {
		list, err := s.ListServices(ctx)
		if err != nil {
			t.Fatal(err)
		}
		byID := map[int64]*int{}
		for _, sv := range list {
			byID[sv.ID] = sv.CommunionWeek
		}
		return byID[morning.ID], byID[evening.ID]
	}
	first, third := 1, 3

	if err := s.SetCommunion(ctx, &morning.ID, &first); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCommunion(ctx, &evening.ID, &third); err != nil {
		t.Fatal(err)
	}
	if m, e := weeks(); m != nil || e == nil || *e != 3 {
		t.Fatalf("choosing the evening should clear the morning: morning = %v, evening = %v", m, e)
	}
	if err := s.SetCommunion(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
	if m, e := weeks(); m != nil || e != nil {
		t.Fatalf("no communion left: morning = %v, evening = %v", m, e)
	}

	fifth := 5
	if err := s.SetCommunion(ctx, &evening.ID, &fifth); !isValidation(err) {
		t.Errorf("week 5: err = %v", err)
	}
	if err := s.SetCommunion(ctx, &evening.ID, nil); !isValidation(err) {
		t.Errorf("missing week: err = %v", err)
	}
	missing := int64(999)
	if err := s.SetCommunion(ctx, &missing, &first); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown service: err = %v", err)
	}
	if m, e := weeks(); m != nil || e != nil {
		t.Fatalf("a failed choice must not clear anything: morning = %v, evening = %v", m, e)
	}
}
