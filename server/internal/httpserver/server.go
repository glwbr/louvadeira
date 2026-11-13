package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"louvadeira/internal/messenger"
	"louvadeira/internal/store"
	"louvadeira/internal/whatsapp"
)

type WhatsApp interface {
	Status() whatsapp.Status
	Groups(ctx context.Context) ([]whatsapp.Group, error)
}

type Deps struct {
	Log          *slog.Logger
	UI           fs.FS
	Store        *store.Store
	Now          func() time.Time
	WhatsAppMode string
	WhatsApp     WhatsApp
	Messenger    messenger.Messenger
	AdminToken   string
}

type healthResponse struct {
	OK       bool           `json:"ok"`
	WhatsApp whatsAppHealth `json:"whatsapp"`
}

type whatsAppHealth struct {
	Mode string `json:"mode"`
	whatsapp.Status
}

func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", d.health)

	mux.HandleFunc("POST /api/auth/redeem", d.redeem)
	mux.HandleFunc("POST /api/auth/logout", d.logout)
	mux.Handle("GET /api/me", d.signedIn(http.HandlerFunc(d.me)))

	admin := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, d.admin(handler))
	}
	admin("GET /api/people", d.listPeople)
	admin("POST /api/people", d.createPerson)
	admin("PUT /api/people/{id}", d.updatePerson)
	admin("DELETE /api/people/{id}", d.archivePerson)

	admin("GET /api/services", d.listServices)
	admin("POST /api/services", d.createService)
	admin("PUT /api/services/{id}", d.updateService)

	admin("GET /api/ministries", d.listMinistries)
	admin("POST /api/ministries", d.createMinistry)
	admin("GET /api/ministries/{id}", d.getMinistry)
	admin("PUT /api/ministries/{id}", d.updateMinistry)
	admin("DELETE /api/ministries/{id}", d.archiveMinistry)
	admin("POST /api/ministries/{id}/roles", d.createRole)
	admin("PUT /api/ministries/{id}/roles/{roleId}", d.updateRole)
	admin("DELETE /api/ministries/{id}/roles/{roleId}", d.deleteRole)
	admin("PUT /api/ministries/{id}/members/{personId}", d.putMember)
	admin("DELETE /api/ministries/{id}/members/{personId}", d.removeMember)
	admin("PUT /api/ministries/{id}/services", d.setMinistryServices)

	admin("GET /api/admin/whatsapp/groups", d.groups)
	admin("POST /api/admin/whatsapp/test-poll", d.testPoll)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "rota não encontrada")
	})
	mux.Handle("/", spa(d.UI))
	return mux
}

func (d Deps) health(w http.ResponseWriter, r *http.Request) {
	status := whatsapp.Status{}
	if d.WhatsApp != nil {
		status = d.WhatsApp.Status()
	}
	writeJSON(w, http.StatusOK, healthResponse{OK: true, WhatsApp: whatsAppHealth{Mode: d.WhatsAppMode, Status: status}})
}

func (d Deps) groups(w http.ResponseWriter, r *http.Request) {
	if d.WhatsApp == nil || !d.WhatsApp.Status().Connected {
		writeError(w, http.StatusServiceUnavailable, "WhatsApp desconectado")
		return
	}
	groups, err := d.WhatsApp.Groups(r.Context())
	if err != nil {
		d.Log.Error("listar grupos", "erro", err)
		writeError(w, http.StatusBadGateway, "não consegui listar os grupos")
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

type testPollRequest struct {
	To messenger.Address `json:"to"`
}

func (d Deps) testPoll(w http.ResponseWriter, r *http.Request) {
	if d.Messenger == nil {
		writeError(w, http.StatusServiceUnavailable, "WhatsApp desligado")
		return
	}
	var req testPollRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.To == "" {
		writeError(w, http.StatusBadRequest, `envie {"to": "<grupo ou número>"}`)
		return
	}
	id, err := d.Messenger.SendPoll(r.Context(), req.To, "Teste da louvadeira: chegou?", []string{"✅ Chegou", "❌ Não chegou"})
	if errors.Is(err, messenger.ErrNotAllowed) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		d.Log.Error("enviar enquete de teste", "erro", err)
		writeError(w, http.StatusBadGateway, "não consegui enviar a enquete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]messenger.MessageID{"id": id})
}

/**
 * NOTE: Serves the built PWA. Unknown paths get index.html so client-side routes like /escala/123 load the app;
 * hashed files under assets/ are cached for a year, everything else is revalidated so updates show up.
 */
func spa(ui fs.FS) http.Handler {
	files := http.FileServerFS(ui)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(ui, name); err != nil || info.IsDir() {
			serveIndex(w, r, ui)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, ui fs.FS) {
	index, err := fs.ReadFile(ui, "index.html")
	if err != nil {
		http.Error(w, "front-end não compilado; rode `mise run web:build`", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(index)
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

/**
 * CONSTRAINT: Requests must be JSON. Together with SameSite=Lax cookies this keeps other sites from posting forms
 * into the API with the leader's session.
 */
func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "envie JSON")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "identificador inválido")
		return 0, false
	}
	return id, true
}

func (d Deps) respond(w http.ResponseWriter, status int, body any, err error) {
	if err != nil {
		d.fail(w, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, body)
}

func (d Deps) fail(w http.ResponseWriter, err error) {
	var validation store.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, validation.Message)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "não encontrado")
	case errors.Is(err, store.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		d.Log.Error("erro inesperado", "erro", err)
		writeError(w, http.StatusInternalServerError, "erro inesperado no servidor")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
