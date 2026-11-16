package httpserver

import (
	"net"
	"net/http"
	"strconv"
	"strings"
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
 * CONSTRAINT: Behind a tunnel or a reverse proxy every request comes from that program on this machine, so the client
 * is the address it reports. The last entry of X-Forwarded-For is the one the proxy or Cloudflare wrote; earlier
 * entries, and a CF-Connecting-IP that arrives through another proxy, come from the visitor, so Cloudflare's header
 * only counts when there is no X-Forwarded-For. Both are only trusted on loopback, where nobody else can set them.
 */
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return host
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if last := strings.TrimSpace(chain[len(chain)-1]); last != "" {
		return last
	}
	if forwarded := r.Header.Get("CF-Connecting-IP"); forwarded != "" {
		return forwarded
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
