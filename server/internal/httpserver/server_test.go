package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"louvadeira/internal/httpserver"
	"louvadeira/internal/messenger"
	"louvadeira/internal/notify"
	"louvadeira/internal/storage"
	"louvadeira/internal/store"
	"louvadeira/internal/whatsapp"
)

type fakeWhatsApp struct{ status whatsapp.Status }

func (f fakeWhatsApp) Status() whatsapp.Status { return f.status }

func (f fakeWhatsApp) Groups(context.Context) ([]whatsapp.Group, error) {
	return []whatsapp.Group{{ID: "1@g.us", Name: "Teste"}}, nil
}

func (f fakeWhatsApp) SendButtonTests(context.Context, messenger.Address) ([]whatsapp.ButtonTestResult, error) {
	return []whatsapp.ButtonTestResult{{Kind: "lista", Sent: true}}, nil
}

func (f fakeWhatsApp) GroupParticipants(context.Context, messenger.Address) ([]whatsapp.Participant, error) {
	return []whatsapp.Participant{{Phone: "5511988887777", Name: "Ana"}}, nil
}

type fakeMessenger struct {
	polls []messenger.Address
	texts []string
}

func (f *fakeMessenger) SendText(_ context.Context, _ messenger.Address, text string) (messenger.MessageID, error) {
	f.texts = append(f.texts, text)
	return "", nil
}

func (f *fakeMessenger) SendMention(_ context.Context, _ messenger.Address, text string, _ []messenger.Address) (messenger.MessageID, error) {
	f.texts = append(f.texts, text)
	return "", nil
}

func (f *fakeMessenger) SendPoll(_ context.Context, to messenger.Address, _ messenger.Poll) (messenger.MessageID, error) {
	f.polls = append(f.polls, to)
	return "poll-1", nil
}

var ui = fstest.MapFS{
	"index.html":           {Data: []byte("<html>app</html>")},
	"assets/app-1.js":      {Data: []byte("console.log(1)")},
	"manifest.webmanifest": {Data: []byte("{}")},
}

var now = time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)

type harness struct {
	t       *testing.T
	handler http.Handler
	store   *store.Store
	polls   *fakeMessenger
	cookie  string
	clock   time.Time
}

func newHarness(t *testing.T) *harness {
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
	h := &harness{t: t, store: store.New(db), polls: &fakeMessenger{}, clock: now}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.handler = httpserver.New(httpserver.Deps{
		Log:          logger,
		UI:           ui,
		Store:        h.store,
		Now:          func() time.Time { return h.clock },
		WhatsAppMode: "test",
		WhatsApp:     fakeWhatsApp{status: whatsapp.Status{Linked: true, Connected: true}},
		Messenger:    h.polls,
		Bot:          &notify.Bot{Store: h.store, Messenger: h.polls, Log: logger, Now: func() time.Time { return h.clock }},
		AdminToken:   "segredo",
		PublicURL:    "https://louvadeira.test",
	})
	return h
}

func (h *harness) do(method, path, body string, headers ...string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if h.cookie != "" {
		req.Header.Set("Cookie", h.cookie)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *harness) signIn(person store.Person) {
	h.t.Helper()
	token, err := h.store.CreateLoginToken(context.Background(), person.ID, now, 15*time.Minute)
	if err != nil {
		h.t.Fatal(err)
	}
	rec := h.do(http.MethodPost, "/api/auth/redeem", fmt.Sprintf(`{"token":%q}`, token))
	if rec.Code != http.StatusOK {
		h.t.Fatalf("redeem: %d %s", rec.Code, rec.Body)
	}
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		h.t.Fatalf("cookie flags = %+v", cookie)
	}
	h.cookie = cookie.Name + "=" + cookie.Value
}

func (h *harness) signInAdmin() store.Person {
	h.t.Helper()
	admin, err := h.store.EnsureAdmin(context.Background(), store.PersonInput{Name: "Admin", Phone: "11988887777"})
	if err != nil {
		h.t.Fatal(err)
	}
	h.signIn(admin)
	return admin
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealthReportsWhatsAppState(t *testing.T) {
	rec := newHarness(t).do(http.MethodGet, "/api/health", "")
	body := decode[struct {
		OK       bool `json:"ok"`
		WhatsApp struct {
			Mode      string `json:"mode"`
			Linked    bool   `json:"linked"`
			Connected bool   `json:"connected"`
		} `json:"whatsapp"`
	}](t, rec)
	if !body.OK || body.WhatsApp.Mode != "test" || !body.WhatsApp.Linked || !body.WhatsApp.Connected {
		t.Fatalf("body = %+v", body)
	}
}

func TestClientRoutesFallBackToIndex(t *testing.T) {
	rec := newHarness(t).do(http.MethodGet, "/escala/123", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("status = %d, Cache-Control = %q, body = %q", rec.Code, rec.Header().Get("Cache-Control"), rec.Body)
	}
}

func TestStaticFilesHaveTheRightHeaders(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodGet, "/assets/app-1.js", ""); !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec := h.do(http.MethodGet, "/manifest.webmanifest", ""); rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest Content-Type = %q", rec.Header().Get("Content-Type"))
	}
}

func TestMissingFrontEndBuildIsExplained(t *testing.T) {
	h := httpserver.New(httpserver.Deps{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), UI: fstest.MapFS{}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	rec := newHarness(t).do(http.MethodGet, "/api/nada", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "error") {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
}

func TestRegistryRequiresAnAdministrator(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodGet, "/api/people", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status = %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/me", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous /me: status = %d", rec.Code)
	}

	member, _ := h.store.CreatePerson(context.Background(), store.PersonInput{Name: "Membro", Phone: "11977776666"})
	h.signIn(member)
	if rec := h.do(http.MethodGet, "/api/me", ""); rec.Code != http.StatusOK || decode[store.Person](t, rec).ID != member.ID {
		t.Fatalf("member /me: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodGet, "/api/people", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("member on admin route: status = %d", rec.Code)
	}
}

func TestLoginLinkWorksOnlyOnce(t *testing.T) {
	h := newHarness(t)
	admin, _ := h.store.EnsureAdmin(context.Background(), store.PersonInput{Name: "Admin", Phone: "11988887777"})
	token, _ := h.store.CreateLoginToken(context.Background(), admin.ID, now, 15*time.Minute)
	body := fmt.Sprintf(`{"token":%q}`, token)
	if rec := h.do(http.MethodPost, "/api/auth/redeem", body); rec.Code != http.StatusOK {
		t.Fatalf("first: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/auth/redeem", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("second: %d", rec.Code)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	if rec := h.do(http.MethodPost, "/api/auth/logout", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/me", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", rec.Code)
	}
}

func TestMutationsRequireJSON(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	rec := h.do(http.MethodPost, "/api/people", "name=Ana", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form post: status = %d", rec.Code)
	}
}

func TestRegistryFlow(t *testing.T) {
	h := newHarness(t)
	admin := h.signInAdmin()

	rec := h.do(http.MethodPost, "/api/ministries", `{"name":"Louvor","rotationUnit":"week"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ministry: %d %s", rec.Code, rec.Body)
	}
	ministry := decode[store.Ministry](t, rec)

	rec = h.do(http.MethodPost, fmt.Sprintf("/api/ministries/%d/roles", ministry.ID), `{"name":"Voz","slots":3}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create role: %d %s", rec.Code, rec.Body)
	}
	role := decode[store.Role](t, rec)

	rec = h.do(http.MethodPost, "/api/people", `{"name":"Bia","phone":"(11) 96666-5555"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create person: %d %s", rec.Code, rec.Body)
	}
	person := decode[store.Person](t, rec)

	rec = h.do(http.MethodPost, "/api/services", `{"name":"Culto de quinta","weekday":4,"startTime":"19:30"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create service: %d %s", rec.Code, rec.Body)
	}
	service := decode[store.Service](t, rec)

	member := fmt.Sprintf(`{"isLeader":false,"roles":[{"roleId":%d,"fixed":false}]}`, role.ID)
	if rec := h.do(http.MethodPut, fmt.Sprintf("/api/ministries/%d/members/%d", ministry.ID, person.ID), member); rec.Code != http.StatusNoContent {
		t.Fatalf("put member: %d %s", rec.Code, rec.Body)
	}
	services := fmt.Sprintf(`{"serviceIds":[%d]}`, service.ID)
	if rec := h.do(http.MethodPut, fmt.Sprintf("/api/ministries/%d/services", ministry.ID), services); rec.Code != http.StatusNoContent {
		t.Fatalf("set services: %d %s", rec.Code, rec.Body)
	}

	rec = h.do(http.MethodGet, fmt.Sprintf("/api/ministries/%d", ministry.ID), "")
	detail := decode[store.MinistryDetail](t, rec)
	if len(detail.Roles) != 1 || len(detail.Members) != 1 || detail.Members[0].Roles[0].RoleID != role.ID || len(detail.ServiceIDs) != 1 {
		t.Fatalf("detail = %+v", detail)
	}

	rec = h.do(http.MethodPost, "/api/people", `{"name":"Repetida","phone":"+55 11 96666-5555"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "telefone") {
		t.Fatalf("duplicate phone: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodDelete, fmt.Sprintf("/api/people/%d", admin.ID), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("self archive: %d", rec.Code)
	}
	if rec := h.do(http.MethodDelete, "/api/people/999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing person: %d", rec.Code)
	}
}

func TestAdminTokenStillWorksForWhatsAppTools(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodPost, "/api/admin/whatsapp/test-poll", `{"to":"1@g.us"}`); rec.Code != http.StatusUnauthorized || len(h.polls.polls) != 0 {
		t.Fatalf("without token: %d, polls = %v", rec.Code, h.polls.polls)
	}
	if rec := h.do(http.MethodPost, "/api/admin/whatsapp/test-poll", `{"to":"1@g.us"}`, "Authorization", "Bearer errado"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", rec.Code)
	}
	rec := h.do(http.MethodPost, "/api/admin/whatsapp/test-poll", `{"to":"1@g.us"}`, "Authorization", "Bearer segredo")
	if rec.Code != http.StatusOK || len(h.polls.polls) != 1 {
		t.Fatalf("with token: %d %s, polls = %v", rec.Code, rec.Body, h.polls.polls)
	}
}

func TestScheduleFlow(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	ctx := context.Background()
	ministry, _ := h.store.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	role, _ := h.store.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	service, _ := h.store.CreateService(ctx, store.ServiceInput{Name: "Quinta", Weekday: 4, StartTime: "19:30"})
	_ = h.store.SetMinistryServices(ctx, ministry.ID, []int64{service.ID})
	bassist, _ := h.store.CreatePerson(ctx, store.PersonInput{Name: "Eli"})
	_ = h.store.PutMember(ctx, ministry.ID, bassist.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: role.ID}}})
	base := fmt.Sprintf("/api/ministries/%d/schedules", ministry.ID)

	if rec := h.do(http.MethodGet, base+"/2026-12", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("before generating: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, base+"/2026-13", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad month: %d", rec.Code)
	}
	rec := h.do(http.MethodPost, base+"/2026-12/generate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body)
	}
	view := decode[store.ScheduleView](t, rec)
	if len(view.Weeks) != 5 || view.Weeks[0].Services[0].Roles[0].Slots[0].PersonName != "Eli" {
		t.Fatalf("view = %+v", view)
	}

	slot := view.Weeks[0].Services[0].Roles[0].Slots[0].AssignmentID
	if rec := h.do(http.MethodPut, fmt.Sprintf("%s/2026-12/assignments/%d", base, slot), `{"personId":null}`); rec.Code != http.StatusNoContent {
		t.Fatalf("clear slot: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodDelete, base+"/2026-12", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("discard draft: %d %s", rec.Code, rec.Body)
	}
	rec = h.do(http.MethodPost, base+"/2026-12/generate", "")
	slot = decode[store.ScheduleView](t, rec).Weeks[0].Services[0].Roles[0].Slots[0].AssignmentID
	if rec := h.do(http.MethodPut, fmt.Sprintf("%s/2026-12/assignments/%d", base, slot), `{"personId":null}`); rec.Code != http.StatusNoContent {
		t.Fatalf("clear slot again: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, base+"/2026-12/publish", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do(http.MethodPost, base+"/2026-12/publish", ""); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "já foi publicada") {
		t.Fatalf("publish twice: %d %s", rec.Code, rec.Body)
	}
	rec = h.do(http.MethodPost, base+"/2026-12/generate", "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "publicada") {
		t.Fatalf("regenerate published: %d %s", rec.Code, rec.Body)
	}
	view = decode[store.ScheduleView](t, h.do(http.MethodGet, base+"/2026-12", ""))
	if view.Status != store.StatusPublished || view.Warnings != 1 {
		t.Fatalf("published view: status %s, warnings %d", view.Status, view.Warnings)
	}

	rec = h.do(http.MethodPost, "/api/unavailabilities", fmt.Sprintf(`{"personId":%d,"startDate":"2026-12-24","endDate":"2026-12-26","reason":"familia"}`, bassist.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create unavailability: %d %s", rec.Code, rec.Body)
	}
	list := decode[[]store.Unavailability](t, h.do(http.MethodGet, "/api/unavailabilities?from=2026-12-01&to=2026-12-31", ""))
	if len(list) != 1 || list[0].PersonName != "Eli" {
		t.Fatalf("unavailabilities = %+v", list)
	}
	if rec := h.do(http.MethodGet, "/api/unavailabilities?from=ontem&to=hoje", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad range: %d", rec.Code)
	}
}

func TestGroupParticipantsRequireAnAdministrator(t *testing.T) {
	h := newHarness(t)
	if rec := h.do(http.MethodGet, "/api/admin/whatsapp/groups/1@g.us/participants", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	h.signInAdmin()
	rec := h.do(http.MethodGet, "/api/admin/whatsapp/groups/1@g.us/participants", "")
	if rec.Code != http.StatusOK || decode[[]whatsapp.Participant](t, rec)[0].Phone != "5511988887777" {
		t.Fatalf("participants: %d %s", rec.Code, rec.Body)
	}
}

func TestDeclineFromTheAppOpensASwap(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	ctx := context.Background()
	ministry, _ := h.store.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	role, _ := h.store.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	service, _ := h.store.CreateService(ctx, store.ServiceInput{Name: "Quinta", Weekday: 4, StartTime: "19:30"})
	_ = h.store.SetMinistryServices(ctx, ministry.ID, []int64{service.ID})
	for _, name := range []string{"Edu", "Fred"} {
		p, _ := h.store.CreatePerson(ctx, store.PersonInput{Name: name, Phone: "+55 11 9" + map[string]string{"Edu": "1111-1111", "Fred": "2222-2222"}[name]})
		_ = h.store.PutMember(ctx, ministry.ID, p.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: role.ID}}})
	}
	base := fmt.Sprintf("/api/ministries/%d/schedules/2026-12", ministry.ID)
	view := decode[store.ScheduleView](t, h.do(http.MethodPost, base+"/generate", ""))
	slot := view.Weeks[0].Services[0].Roles[0].Slots[0]

	if rec := h.do(http.MethodPost, fmt.Sprintf("%s/assignments/%d/decline", base, slot.AssignmentID), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("decline on a draft: %d %s", rec.Code, rec.Body)
	}
	h.do(http.MethodPost, base+"/publish", "")
	rec := h.do(http.MethodPost, fmt.Sprintf("%s/assignments/%d/decline", base, slot.AssignmentID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("decline: %d %s", rec.Code, rec.Body)
	}
	covers := decode[[]store.CoverRequest](t, h.do(http.MethodGet, base+"/covers", ""))
	if len(covers) != 1 || covers[0].AbsentName != slot.PersonName || len(covers[0].Invites) != 1 || covers[0].Status != store.CoverOpen {
		t.Fatalf("covers = %+v", covers)
	}
	if len(h.polls.polls) != 1 {
		t.Fatalf("invites sent = %v", h.polls.polls)
	}
}

func TestAbsenceOffersSwapsForTheWholePeriod(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	ctx := context.Background()
	ministry, _ := h.store.CreateMinistry(ctx, store.MinistryInput{Name: "Louvor"})
	role, _ := h.store.CreateRole(ctx, ministry.ID, store.RoleInput{Name: "Baixo", Slots: 1})
	service, _ := h.store.CreateService(ctx, store.ServiceInput{Name: "Quinta", Weekday: 4, StartTime: "19:30"})
	_ = h.store.SetMinistryServices(ctx, ministry.ID, []int64{service.ID})
	for _, name := range []string{"Edu", "Fred"} {
		p, _ := h.store.CreatePerson(ctx, store.PersonInput{Name: name, Phone: "+55 11 9" + map[string]string{"Edu": "1111-1111", "Fred": "2222-2222"}[name]})
		_ = h.store.PutMember(ctx, ministry.ID, p.ID, store.MemberInput{Roles: []store.MemberRole{{RoleID: role.ID}}})
	}
	base := fmt.Sprintf("/api/ministries/%d/schedules/2026-12", ministry.ID)
	view := decode[store.ScheduleView](t, h.do(http.MethodPost, base+"/generate", ""))
	h.do(http.MethodPost, base+"/publish", "")
	absent := view.Weeks[0].Services[0].Roles[0].Slots[0]

	rec := h.do(http.MethodPost, "/api/unavailabilities",
		fmt.Sprintf(`{"personId":%d,"startDate":"2026-12-01","endDate":"2026-12-31","reason":"viagem"}`, absent.PersonID))
	created := decode[struct {
		ID        int64                `json:"id"`
		Scheduled int                  `json:"scheduled"`
		Covers    []store.CoverRequest `json:"covers"`
	}](t, rec)
	if len(created.Covers) < 2 || created.Scheduled != 0 {
		t.Fatalf("created = %+v, want a swap for every December Thursday of %s and none left", created, absent.PersonName)
	}
	if slots := decode[[]map[string]any](t, h.do(http.MethodGet, fmt.Sprintf("/api/unavailabilities/%d/assignments", created.ID), "")); len(slots) != 0 {
		t.Fatalf("assignments still held = %v", slots)
	}
	if requests := decode[[]store.CoverRequest](t, h.do(http.MethodPost, fmt.Sprintf("/api/unavailabilities/%d/covers", created.ID), "")); len(requests) != 0 {
		t.Fatalf("opening the swaps again = %+v", requests)
	}
	if len(h.polls.polls) != 1 {
		t.Fatalf("the other bassist should get one invite for every Thursday, got %v", h.polls.polls)
	}
	list := decode[[]struct {
		Scheduled int `json:"scheduled"`
	}](t, h.do(http.MethodGet, "/api/unavailabilities?from=2026-12-01&to=2026-12-31", ""))
	if len(list) != 1 || list[0].Scheduled != 0 {
		t.Fatalf("after opening swaps the absence still lists %+v", list)
	}
}

func TestCommunionIsChosenOnceForTheChurch(t *testing.T) {
	h := newHarness(t)
	rec := h.do(http.MethodPut, "/api/communion", `{"serviceId":null,"week":null}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	h.signInAdmin()
	service := decode[store.Service](t, h.do(http.MethodPost, "/api/services", `{"name":"Culto de domingo","weekday":0,"startTime":"19:00"}`))

	if rec := h.do(http.MethodPut, "/api/communion", fmt.Sprintf(`{"serviceId":%d,"week":1}`, service.ID)); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	services := decode[[]store.Service](t, h.do(http.MethodGet, "/api/services", ""))
	if services[0].CommunionWeek == nil || *services[0].CommunionWeek != 1 {
		t.Fatalf("services = %+v", services)
	}
	if rec := h.do(http.MethodPut, "/api/communion", fmt.Sprintf(`{"serviceId":%d,"week":9}`, service.ID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad week: %d", rec.Code)
	}
	if rec := h.do(http.MethodPut, "/api/communion", `{"serviceId":999,"week":1}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown service: %d", rec.Code)
	}
}

func TestWritesFromAnotherSiteAreTurnedAway(t *testing.T) {
	h := newHarness(t)
	h.signInAdmin()
	if rec := h.do(http.MethodPost, "/api/auth/logout", "", "Sec-Fetch-Site", "cross-site"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site write: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/people", "", "Sec-Fetch-Site", "cross-site"); rec.Code != http.StatusOK {
		t.Fatalf("cross-site read: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/services", `{"name":"Culto de sábado","weekday":6,"startTime":"18:00"}`, "Sec-Fetch-Site", "same-origin"); rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("same-origin write: %d %s", rec.Code, rec.Body)
	}
}
