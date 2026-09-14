package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Meizuno/calories/internal/service"
)

// The SPA translates the `code`, so these mappings are effectively the API's
// user-facing contract: changing one changes what a user reads.
func TestAuthErrorCodes(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{service.ErrBadCredentials, http.StatusUnauthorized, "bad_credentials"},
		{service.ErrNoPassword, http.StatusUnauthorized, "no_password"},
		{service.ErrEmailTaken, http.StatusConflict, "email_taken"},
		{service.ErrWeakPassword, http.StatusBadRequest, "weak_password"},
		{service.ErrLongPassword, http.StatusBadRequest, "long_password"},
		{service.ErrBadEmail, http.StatusBadRequest, "bad_email"},
		{service.ErrInvalidToken, http.StatusUnauthorized, "session_expired"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			w := httptest.NewRecorder()
			authError(w, c.err)
			if w.Code != c.status {
				t.Fatalf("status = %d, want %d", w.Code, c.status)
			}
			var body struct{ Code, Message string }
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if body.Code != c.code {
				t.Fatalf("code = %q, want %q", body.Code, c.code)
			}
			if body.Message == "" {
				t.Fatal("message is empty")
			}
		})
	}
}

func TestAuthErrorHidesUnknownFailures(t *testing.T) {
	w := httptest.NewRecorder()
	secret := errors.New("pq: password authentication failed for user calories_user")
	authError(w, secret)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// An unrecognised error must not reach the client verbatim — it can carry
	// connection strings or internal detail.
	if strings.Contains(w.Body.String(), "calories_user") {
		t.Fatalf("internal error leaked to the client: %s", w.Body.String())
	}
	var body struct{ Code string }
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body.Code != "unknown" {
		t.Fatalf("code = %q, want unknown", body.Code)
	}
}

func TestOAuthCode(t *testing.T) {
	cases := map[error]string{
		errOAuthState:                "oauth_state",
		errOAuthCancelled:            "oauth_cancelled",
		errOAuthUnverified:           "oauth_unverified",
		errOAuthRejected:             "oauth_failed",
		errors.New("something else"): "oauth_failed",
	}
	for err, want := range cases {
		if got := oauthCode(err); got != want {
			t.Errorf("oauthCode(%v) = %q, want %q", err, got, want)
		}
	}
	// Wrapped errors must still map, since handlers may add context.
	if got := oauthCode(fmt.Errorf("exchange: %w", errOAuthCancelled)); got != "oauth_cancelled" {
		t.Errorf("wrapped error mapped to %q", got)
	}
}

// returnPath guards against an open redirect: anything that could send the
// browser to another origin must collapse to "/".
func TestReturnPath(t *testing.T) {
	cases := map[string]string{
		"":                     "/",
		"/":                    "/",
		"/stats":               "/stats",
		"/log?date=2026-09-14": "/log?date=2026-09-14",
		"//evil.com":           "/", // scheme-relative
		"///evil.com":          "/",
		"https://evil.com":     "/",
		"http://evil.com/x":    "/",
		"evil.com":             "/", // no leading slash
		"javascript:alert(1)":  "/",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/auth/google?return="+urlEscape(in), nil)
			if got := returnPath(r); got != want {
				t.Fatalf("returnPath(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func urlEscape(s string) string {
	r := strings.NewReplacer("?", "%3F", "&", "%26", "=", "%3D", ":", "%3A", "/", "%2F")
	return r.Replace(s)
}
