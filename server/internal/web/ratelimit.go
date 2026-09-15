package web

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Meizuno/calories/internal/ratelimit"
	"github.com/go-chi/chi/v5"
)

// Limits holds the rate limiters the router applies. They are built once and
// shared, which matters because the auth routes are mounted twice (versioned
// and not) and two limiters would quietly double everyone's allowance.
type Limits struct {
	// Auth guards the endpoints that verify a password. Each attempt costs a
	// bcrypt hash, so leaving them open is a cheap way to burn the server's CPU
	// as well as to guess credentials.
	Auth *ratelimit.Limiter
	// Refresh is far more generous: rotating a session is normal traffic, and
	// several tabs waking at once must not lock someone out of their own app.
	Refresh *ratelimit.Limiter
	// Assistant caps chat messages. Unlike the others this is keyed by profile,
	// not by address: the cost follows the account, and one person on a shared
	// network must not exhaust another's allowance.
	Assistant *ratelimit.Limiter
	// TrustProxy says whether X-Forwarded-For may be believed. See clientIP.
	TrustProxy bool
}

// clientIP identifies the caller a limit is counted against.
//
// Behind a reverse proxy the peer address is the proxy, so without this every
// caller would share one bucket and the first person to mistype a password
// would lock out everyone. X-Forwarded-For carries the real address — but only
// a proxy we control may be believed, because anyone can send that header.
//
// Caddy APPENDS the peer it actually saw, so the LAST entry is the one it
// vouches for; earlier entries are whatever the client claimed. Taking the
// first — which is the common shortcut — hands every attacker a fresh bucket
// per request by simply varying the header.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limit refuses a caller who has spent their allowance, with Retry-After so a
// well-behaved client knows when to come back rather than hammering.
func limit(l *ratelimit.Limiter, trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l == nil {
				next.ServeHTTP(w, r)
				return
			}
			ok, retry := l.Allow(clientIP(r, trustProxy))
			if !ok {
				// Nothing has read the body yet, so drain it or the response
				// never reaches the caller (see drain).
				drain(r)
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "too_many_requests",
					"too many attempts — wait a moment and try again")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitByProfile counts against the signed-in account rather than the address.
// It must be mounted INSIDE the gate, which is what puts the profile in the
// context; before it, every caller would share the empty key.
func limitByProfile(l *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l == nil {
				next.ServeHTTP(w, r)
				return
			}
			ok, retry := l.Allow(strconv.FormatInt(ProfileID(r.Context()), 10))
			if !ok {
				drain(r)
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "too_many_requests",
					"you have used this hour's messages -- try again shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limited is sugar for mounting a single route behind a limiter.
func limited(r chi.Router, l *ratelimit.Limiter, trustProxy bool) chi.Router {
	return r.With(limit(l, trustProxy))
}
