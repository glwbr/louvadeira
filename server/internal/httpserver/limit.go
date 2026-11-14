package httpserver

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// NOTE: Past this many clients a request also clears the ones that went quiet, so the map stays small.
const sweepAbove = 1000

/**
 * NOTE: A sliding-window limit per client for the routes anyone can reach without signing in. It lives in memory, so
 * a restart forgets it, which is enough to slow down someone hammering the sign-in form.
 */
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) allow(client string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.recent(client, now)
	if len(recent) >= l.limit {
		l.hits[client] = recent
		return false
	}
	l.hits[client] = append(recent, now)
	if len(l.hits) > sweepAbove {
		for other := range l.hits {
			if len(l.recent(other, now)) == 0 {
				delete(l.hits, other)
			}
		}
	}
	return true
}

func (l *limiter) recent(client string, now time.Time) []time.Time {
	var recent []time.Time
	for _, t := range l.hits[client] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	return recent
}

/**
 * CONSTRAINT: Through the Cloudflare tunnel every request comes from cloudflared on this machine, so the client is the
 * address Cloudflare reports. That header is only trusted on loopback, where nobody else can set it.
 */
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if forwarded := r.Header.Get("CF-Connecting-IP"); forwarded != "" {
			return forwarded
		}
	}
	return host
}

func (d Deps) limited(l *limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), d.now()) {
			w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
			writeError(w, http.StatusTooManyRequests, "muitas tentativas seguidas; espere alguns minutos e tente de novo")
			return
		}
		next(w, r)
	}
}
