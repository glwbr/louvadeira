package httpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"louvadeira/internal/store"
)

const (
	sessionCookie = "lv_session"
	SessionTTL    = 90 * 24 * time.Hour
)

type personKey struct{}

func personFrom(ctx context.Context) (store.Person, bool) {
	p, ok := ctx.Value(personKey{}).(store.Person)
	return p, ok
}

type redeemRequest struct {
	Token string `json:"token"`
}

func (d Deps) redeem(w http.ResponseWriter, r *http.Request) {
	var req redeemRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	session, person, err := d.Store.RedeemLoginToken(r.Context(), req.Token, d.now(), SessionTTL)
	if err != nil {
		d.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    session,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, person)
}

func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := d.Store.DeleteSession(r.Context(), cookie.Value); err != nil {
			d.Log.Error("encerrar sessão", "erro", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) me(w http.ResponseWriter, r *http.Request) {
	person, _ := personFrom(r.Context())
	writeJSON(w, http.StatusOK, person)
}

func (d Deps) signedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "entre pelo link de acesso")
			return
		}
		person, err := d.Store.SessionPerson(r.Context(), cookie.Value, d.now())
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "sessão expirada; peça um novo link de acesso")
			return
		}
		if err != nil {
			d.fail(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), personKey{}, person)))
	})
}

/**
 * NOTE: Administrators come in through a session cookie (the app) or the shared admin token (curl on the server).
 */
func (d Deps) admin(next http.Handler) http.Handler {
	requireAdmin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if person, _ := personFrom(r.Context()); !person.IsAdmin {
			writeError(w, http.StatusForbidden, "só administradores podem fazer isso; peça a um deles")
			return
		}
		next.ServeHTTP(w, r)
	})
	viaSession := d.signedIn(requireAdmin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if d.AdminToken == "" || subtle.ConstantTimeCompare([]byte(bearer), []byte(d.AdminToken)) != 1 {
				writeError(w, http.StatusUnauthorized, "token de administrador inválido")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		viaSession.ServeHTTP(w, r)
	})
}

// NOTE: Behind Caddy the TLS ends at the proxy, so the forwarded scheme decides whether cookies are Secure.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
