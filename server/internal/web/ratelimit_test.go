package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Meizuno/calories/internal/ratelimit"
)

func request(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remote     string
		headers    map[string]string
		trustProxy bool
		want       string
	}{
		{
			name:   "no proxy: the peer address, without its port",
			remote: "203.0.113.7:54321",
			want:   "203.0.113.7",
		},
		{
			// The whole reason the flag exists. Exposed directly, anyone could
			// hand themselves a fresh bucket per request by varying the header.
			name:    "untrusted X-Forwarded-For is ignored",
			remote:  "203.0.113.7:54321",
			headers: map[string]string{"X-Forwarded-For": "1.1.1.1"},
			want:    "203.0.113.7",
		},
		{
			name:       "trusted proxy: the forwarded address",
			remote:     "10.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7"},
			trustProxy: true,
			want:       "203.0.113.7",
		},
		{
			// Caddy appends the peer it actually saw, so the last entry is the
			// one it vouches for. Taking the first — the common shortcut — would
			// return the attacker's own invention here.
			name:       "trusted proxy: a spoofed prefix does not win",
			remote:     "10.0.0.1:8080",
			headers:    map[string]string{"X-Forwarded-For": "9.9.9.9, 8.8.8.8, 203.0.113.7"},
			trustProxy: true,
			want:       "203.0.113.7",
		},
		{
			name:       "trusted proxy: X-Real-IP when there is no XFF",
			remote:     "10.0.0.1:8080",
			headers:    map[string]string{"X-Real-IP": "203.0.113.7"},
			trustProxy: true,
			want:       "203.0.113.7",
		},
		{
			name:       "trusted proxy: falls back to the peer when no header is set",
			remote:     "203.0.113.7:443",
			trustProxy: true,
			want:       "203.0.113.7",
		},
		{
			name:       "an empty forwarded header does not become an empty key",
			remote:     "203.0.113.7:443",
			headers:    map[string]string{"X-Forwarded-For": "   "},
			trustProxy: true,
			want:       "203.0.113.7",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clientIP(request(c.remote, c.headers), c.trustProxy); got != c.want {
				t.Errorf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLimitMiddleware(t *testing.T) {
	served := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	})

	t.Run("refuses past the burst, with a code and a Retry-After", func(t *testing.T) {
		served = 0
		h := limit(ratelimit.New(3, time.Minute), false)(next)

		for i := 1; i <= 3; i++ {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, request("203.0.113.7:1", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("request %d = %d, want 200", i, w.Code)
			}
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, request("203.0.113.7:1", nil))
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", w.Code)
		}
		if served != 3 {
			t.Errorf("handler ran %d times, want 3 — the refused request must not reach it", served)
		}
		var body struct{ Code, Message string }
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not the {code,message} shape: %q", w.Body.String())
		}
		if body.Code != "too_many_requests" {
			t.Errorf("code = %q", body.Code)
		}
		retry := w.Header().Get("Retry-After")
		if n, err := strconv.Atoi(retry); err != nil || n < 1 {
			t.Errorf("Retry-After = %q, want a positive whole number of seconds", retry)
		}
	})

	t.Run("one caller cannot lock out another", func(t *testing.T) {
		served = 0
		h := limit(ratelimit.New(2, time.Minute), false)(next)
		for i := 0; i < 5; i++ {
			h.ServeHTTP(httptest.NewRecorder(), request("203.0.113.7:1", nil))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, request("198.51.100.4:1", nil))
		if w.Code != http.StatusOK {
			t.Errorf("an unrelated caller got %d, want 200", w.Code)
		}
	})

	t.Run("a nil limiter is a no-op rather than a closed door", func(t *testing.T) {
		served = 0
		h := limit(nil, false)(next)
		for i := 0; i < 20; i++ {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, request("203.0.113.7:1", nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
		}
	})
}
