package httpserver_test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"louvadeira/internal/store"
)

func setUpBassists(t *testing.T, h *harness) (store.Person, store.Person) {
	t.Helper()
	ctx := context.Background()
	ministry, _ := h.store.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	role, _ := h.store.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	service, _ := h.store.CreateService(ctx, store.ServiceInput{Name: "Quinta", Weekday: 4, StartTime: "19:30"})
	_ = h.store.SetMinistryServices(ctx, ministry.ID, []int64{service.ID})
	edu, _ := h.store.CreatePerson(ctx, store.PersonInput{Name: "Edu", Phone: "+55 11 91111-1111"})
	fred, _ := h.store.CreatePerson(ctx, store.PersonInput{Name: "Fred", Phone: "+55 11 92222-2222"})
	for _, p := range []store.Person{edu, fred} {
		_ = h.store.PutMember(ctx, ministry.ID, p.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: role.ID}}})
	}
	if err := h.store.GenerateDraft(ctx, ministry.ID, "2026-12", now); err != nil {
		t.Fatal(err)
	}
	if err := h.store.PublishSchedule(ctx, ministry.ID, "2026-12", now); err != nil {
		t.Fatal(err)
	}
	return edu, fred
}

func TestMembersSeeAndAnswerOnlyTheirOwnServices(t *testing.T) {
	h := newHarness(t)
	edu, fred := setUpBassists(t, h)
	fredSlots, _ := h.store.MemberSlots(context.Background(), fred.ID, "2026-12-01", "2026-12-31")
	h.signIn(edu)

	if rec := h.do(http.MethodGet, "/api/people", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("member reached the admin area: %d", rec.Code)
	}
	slots := decode[[]store.MemberSlot](t, h.do(http.MethodGet, "/api/me/assignments", ""))
	if len(slots) < 2 {
		t.Fatalf("slots = %+v", slots)
	}
	first, second := slots[0].AssignmentID, slots[1].AssignmentID

	if rec := h.do(http.MethodPost, "/api/me/confirm", fmt.Sprintf(`{"assignmentIds":[%d]}`, fredSlots[0].AssignmentID)); rec.Code != http.StatusNotFound {
		t.Fatalf("confirming someone else's service: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, "/api/me/confirm", fmt.Sprintf(`{"assignmentIds":[%d]}`, first)); rec.Code != http.StatusNoContent {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, "/api/me/decline", fmt.Sprintf(`{"assignmentIds":[%d]}`, second)); rec.Code != http.StatusOK {
		t.Fatalf("decline: %d %s", rec.Code, rec.Body)
	}
	slots = decode[[]store.MemberSlot](t, h.do(http.MethodGet, "/api/me/assignments", ""))
	if slots[0].Status != store.AssignmentConfirmed || slots[1].Status != store.AssignmentDeclined || !slots[1].CoverOpen {
		t.Fatalf("after answering = %+v", slots[:2])
	}
	if len(h.polls.polls) != 1 {
		t.Fatalf("Fred should be invited once, polls = %v", h.polls.polls)
	}

	rec := h.do(http.MethodPost, "/api/me/unavailabilities",
		fmt.Sprintf(`{"personId":%d,"startDate":"2026-12-01","repeat":"weekly","weekday":4,"reason":"estudos"}`, fred.ID))
	created := decode[struct {
		ID        int64  `json:"id"`
		PersonID  int64  `json:"personId"`
		EndDate   string `json:"endDate"`
		Scheduled int    `json:"scheduled"`
	}](t, rec)
	if created.PersonID != edu.ID || created.EndDate != "" || created.Scheduled != len(slots)-1 {
		t.Fatalf("created = %+v (slots %d)", created, len(slots))
	}
	other, _ := h.store.CreateUnavailability(context.Background(), store.UnavailabilityInput{PersonID: fred.ID, StartDate: "2026-12-24", Reason: store.ReasonTravel})
	if rec := h.do(http.MethodDelete, fmt.Sprintf("/api/me/unavailabilities/%d", other.ID), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deleting someone else's absence: %d", rec.Code)
	}
	mine := decode[[]struct {
		ID int64 `json:"id"`
	}](t, h.do(http.MethodGet, "/api/me/unavailabilities", ""))
	if len(mine) != 1 || mine[0].ID != created.ID {
		t.Fatalf("my absences = %+v", mine)
	}
}

var linkPattern = regexp.MustCompile(`https://louvadeira\.test/entrar\?token=([\w-]+)`)

func TestAccessLinkArrivesOnWhatsApp(t *testing.T) {
	h := newHarness(t)
	setUpBassists(t, h)

	if rec := h.do(http.MethodPost, "/api/auth/link", `{"phone":"123"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid phone: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/auth/link", `{"phone":"(11) 93333-3333"}`); rec.Code != http.StatusAccepted || len(h.polls.texts) != 0 {
		t.Fatalf("unknown phone: %d, texts %v", rec.Code, h.polls.texts)
	}
	if rec := h.do(http.MethodPost, "/api/auth/link", `{"phone":"(11) 91111-1111"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("known phone: %d", rec.Code)
	}
	h.do(http.MethodPost, "/api/auth/link", `{"phone":"11 91111-1111"}`)
	if len(h.polls.texts) != 1 || !strings.Contains(h.polls.texts[0], "Oi, Edu!") {
		t.Fatalf("texts = %v", h.polls.texts)
	}
	match := linkPattern.FindStringSubmatch(h.polls.texts[0])
	if match == nil {
		t.Fatalf("no link in %q", h.polls.texts[0])
	}
	if rec := h.do(http.MethodPost, "/api/auth/redeem", fmt.Sprintf(`{"token":%q}`, match[1])); rec.Code != http.StatusOK {
		t.Fatalf("redeem: %d %s", rec.Code, rec.Body)
	}
}
