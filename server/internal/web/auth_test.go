package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Meizuno/calories/internal/service"
)

func testAuth(t *testing.T) *Auth {
	t.Helper()
	// A nil *db.Queries is fine: only the token methods are exercised here.
	return NewAuth(service.NewAuth(nil, "test-secret-at-least-32-characters-long", time.Minute, time.Hour), false)
}

func TestBearer(t *testing.T) {
	cases := []struct {
		name  string
		build func(*http.Request)
		want  string
	}{
		{"nothing", func(*http.Request) {}, ""},
		{"authorization header", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer abc.def.ghi")
		}, "abc.def.ghi"},
		{"header is trimmed", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer   abc  ")
		}, "abc"},
		{"cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: accessCookie, Value: "cookie-token"})
		}, "cookie-token"},
		{"empty cookie is not a token", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: accessCookie, Value: ""})
		}, ""},
		{"non-bearer scheme ignored", func(r *http.Request) {
			r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		}, ""},
		// A PAT arrives in the same header; bearer() hands it back and the caller
		// tells the two apart by prefix.
		{"PAT is returned verbatim", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+service.PATPrefix+"xyz")
		}, service.PATPrefix + "xyz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/day", nil)
			c.build(r)
			if got := bearer(r); got != c.want {
				t.Fatalf("bearer() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBearerPrefersHeaderOverCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/day", nil)
	r.Header.Set("Authorization", "Bearer from-header")
	r.AddCookie(&http.Cookie{Name: accessCookie, Value: "from-cookie"})
	if got := bearer(r); got != "from-header" {
		t.Fatalf("bearer() = %q, want the header to win", got)
	}
}

func TestResolveIgnoresPAT(t *testing.T) {
	a := testAuth(t)
	r := httptest.NewRequest(http.MethodGet, "/api/day", nil)
	r.Header.Set("Authorization", "Bearer "+service.PATPrefix+"something")
	// A PAT is not a session: Resolve must not treat it as one, or a scoped
	// token would silently gain full-session access.
	if uid := a.Resolve(r); uid != "" {
		t.Fatalf("Resolve() = %q for a PAT, want empty", uid)
	}
}

func TestResolveRoundTrip(t *testing.T) {
	a := testAuth(t)
	tok, err := a.svc.SignAccess("user-9")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/day", nil)
	r.AddCookie(&http.Cookie{Name: accessCookie, Value: tok})
	if uid := a.Resolve(r); uid != "user-9" {
		t.Fatalf("Resolve() = %q, want user-9", uid)
	}
}

func TestSetSessionCookies(t *testing.T) {
	a := testAuth(t)
	w := httptest.NewRecorder()
	if err := a.SetSession(w, "user-1", "refresh-value"); err != nil {
		t.Fatalf("SetSession: %v", err)
	}
	got := map[string]*http.Cookie{}
	for _, c := range w.Result().Cookies() {
		got[c.Name] = c
	}

	access, ok := got[accessCookie]
	if !ok {
		t.Fatal("no access cookie")
	}
	if access.Path != "/" || !access.HttpOnly || access.SameSite != http.SameSiteLaxMode {
		t.Fatalf("access cookie attrs wrong: %+v", access)
	}
	if access.Value == "" {
		t.Fatal("access cookie is empty")
	}

	refresh, ok := got[refreshCookie]
	if !ok {
		t.Fatal("no refresh cookie")
	}
	// Scoping the refresh token keeps it off ordinary API traffic; widening this
	// path would send the long-lived credential with every request.
	if refresh.Path != refreshPath {
		t.Fatalf("refresh cookie path = %q, want %q", refresh.Path, refreshPath)
	}
	if !refresh.HttpOnly {
		t.Fatal("refresh cookie must be HttpOnly")
	}
	if refresh.Value != "refresh-value" {
		t.Fatalf("refresh cookie value = %q", refresh.Value)
	}
}

func TestSecureCookiesFollowConfig(t *testing.T) {
	for _, secure := range []bool{true, false} {
		svc := service.NewAuth(nil, "test-secret-at-least-32-characters-long", time.Minute, time.Hour)
		w := httptest.NewRecorder()
		if err := NewAuth(svc, secure).SetSession(w, "u", "r"); err != nil {
			t.Fatalf("SetSession: %v", err)
		}
		for _, c := range w.Result().Cookies() {
			if c.Secure != secure {
				t.Fatalf("cookie %s Secure = %v, want %v", c.Name, c.Secure, secure)
			}
		}
	}
}

func TestClearSessionExpiresBothCookies(t *testing.T) {
	a := testAuth(t)
	w := httptest.NewRecorder()
	a.ClearSession(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cleared %d cookies, want 2", len(cookies))
	}
	paths := map[string]string{accessCookie: "/", refreshCookie: refreshPath}
	for _, c := range cookies {
		if c.Value != "" || c.MaxAge >= 0 {
			t.Fatalf("cookie %s not expired: value=%q maxAge=%d", c.Name, c.Value, c.MaxAge)
		}
		// The browser only drops a cookie when the path matches the one it was
		// set with, so a mismatch here would leave the session alive.
		if want := paths[c.Name]; c.Path != want {
			t.Fatalf("cookie %s path = %q, want %q", c.Name, c.Path, want)
		}
	}
}

func TestGateScope(t *testing.T) {
	full := context.WithValue(context.Background(), fullKey, true)
	pat := context.WithValue(context.Background(), scopesKey, []string{"read"})

	cases := []struct {
		name  string
		ctx   context.Context
		scope string
		want  bool
	}{
		{"full session passes a scoped route", full, "read", true},
		{"full session passes a session-only route", full, "", true},
		{"PAT with the scope passes", pat, "read", true},
		{"PAT without the scope is refused", pat, "add", false},
		// Default-deny: an operation with no declared scope is session-only, so a
		// PAT can never reach update/delete/account management.
		{"PAT cannot reach a session-only route", pat, "", false},
		{"anonymous is refused", context.Background(), "read", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := allowed(c.ctx, c.scope); got != c.want {
				t.Fatalf("allowed(scope=%q) = %v, want %v", c.scope, got, c.want)
			}
		})
	}
}
