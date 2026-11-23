package httpserver_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

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
		ID        int64                `json:"id"`
		PersonID  int64                `json:"personId"`
		EndDate   string               `json:"endDate"`
		Scheduled int                  `json:"scheduled"`
		Covers    []store.CoverRequest `json:"covers"`
	}](t, rec)
	if created.PersonID != edu.ID || created.EndDate != "" || created.Scheduled != 0 || len(created.Covers) != len(slots)-1 {
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

func TestAnotherLinkWaitsUntilTheLastOneIsUsedOrExpires(t *testing.T) {
	h := newHarness(t)
	setUpBassists(t, h)
	ask := func() {
		t.Helper()
		if rec := h.do(http.MethodPost, "/api/auth/link", `{"phone":"(11) 91111-1111"}`); rec.Code != http.StatusAccepted {
			t.Fatalf("ask: %d %s", rec.Code, rec.Body)
		}
	}

	ask()
	h.clock = now.Add(30 * time.Minute)
	ask()
	if len(h.polls.texts) != 1 {
		t.Fatalf("a second link went out while the first still works: %v", h.polls.texts)
	}

	h.clock = now.Add(61 * time.Minute)
	ask()
	if len(h.polls.texts) != 2 {
		t.Fatalf("no new link after the first one expired: %v", h.polls.texts)
	}
	match := linkPattern.FindStringSubmatch(h.polls.texts[1])
	if rec := h.do(http.MethodPost, "/api/auth/redeem", fmt.Sprintf(`{"token":%q}`, match[1])); rec.Code != http.StatusOK {
		t.Fatalf("redeem: %d %s", rec.Code, rec.Body)
	}

	h.clock = h.clock.Add(3 * time.Minute)
	ask()
	if len(h.polls.texts) != 3 {
		t.Fatalf("no new link after the last one was used: %v", h.polls.texts)
	}
}

func TestTheSignInFormSlowsDownOneClient(t *testing.T) {
	h := newHarness(t)
	send := func(remote, forwarded string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/link", strings.NewReader(`{"phone":"(11) 93333-3333"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remote
		if forwarded != "" {
			req.Header.Set("CF-Connecting-IP", forwarded)
		}
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		return rec
	}

	for range 10 {
		if rec := send("127.0.0.1:5000", "203.0.113.7"); rec.Code != http.StatusAccepted {
			t.Fatalf("within the limit: %d", rec.Code)
		}
	}
	if rec := send("127.0.0.1:5000", "203.0.113.7"); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("past the limit: %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := send("127.0.0.1:5000", "203.0.113.8"); rec.Code != http.StatusAccepted {
		t.Fatalf("another visitor through the tunnel was blocked: %d", rec.Code)
	}

	for i := range 10 {
		send("192.168.0.20:4000", fmt.Sprintf("198.51.100.%d", i))
	}
	if rec := send("192.168.0.20:4000", "198.51.100.99"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a header sent from outside the tunnel chose the client: %d", rec.Code)
	}

	h.clock = now.Add(15 * time.Minute)
	if rec := send("127.0.0.1:5000", "203.0.113.7"); rec.Code != http.StatusAccepted {
		t.Fatalf("still blocked after the window: %d", rec.Code)
	}
}

func TestTheSignInFormTellsVisitorsApartBehindAReverseProxy(t *testing.T) {
	h := newHarness(t)
	send := func(forwardedFor, cloudflare string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/link", strings.NewReader(`{"phone":"(11) 93333-3333"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:5000"
		req.Header.Set("X-Forwarded-For", forwardedFor)
		if cloudflare != "" {
			req.Header.Set("CF-Connecting-IP", cloudflare)
		}
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		return rec
	}

	for i := range 10 {
		if rec := send(fmt.Sprintf("198.51.100.%d, 203.0.113.7", i), ""); rec.Code != http.StatusAccepted {
			t.Fatalf("within the limit: %d", rec.Code)
		}
	}
	if rec := send("198.51.100.99, 203.0.113.7", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("an address the visitor wrote chose the client: %d", rec.Code)
	}
	if rec := send("203.0.113.7", "198.51.100.99"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a Cloudflare header the visitor wrote chose the client: %d", rec.Code)
	}
	if rec := send("203.0.113.8", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("another visitor through the proxy was blocked: %d", rec.Code)
	}
}
