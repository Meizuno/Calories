package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type ctxKey int

const (
	userIDKey ctxKey = iota
	profileIDKey
)

// UserID returns the authenticated external user id placed in the context by Auth.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// ProfileID returns the local profile id placed in the context by ProfileResolver.
func ProfileID(ctx context.Context) int64 {
	v, _ := ctx.Value(profileIDKey).(int64)
	return v
}

// Auth resolves the current user via the meizuno SSO (/validate), falling back
// to a dev user id when no token is present or no validate URL is configured.
type Auth struct {
	validateURL string
	refreshURL  string
	devUser     string
	client      *http.Client
}

func NewAuth(validateURL, refreshURL, devUser string) *Auth {
	return &Auth{validateURL: validateURL, refreshURL: refreshURL, devUser: devUser, client: &http.Client{Timeout: 10 * time.Second}}
}

// Middleware gates protected routes: it 401s when there is no session (no valid
// token and no dev fallback), so the SPA can redirect the browser to the login URL.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := a.ResolveWithRefresh(w, r)
		if uid == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Resolve returns the external user id for the request, or "" if anonymous. Used
// by the auth middleware (protected routes) and by the public session endpoint.
func (a *Auth) Resolve(r *http.Request) string {
	if tok := bearer(r); tok != "" && a.validateURL != "" {
		if uid := a.validate(r.Context(), tok); uid != "" {
			return uid
		}
	}
	if a.devUser != "" {
		// Local dev has no real session — honour a simulated logout (set by the
		// logout handler) so the login/logout UX is testable without central auth.
		if c, err := r.Cookie("dev_logout"); err == nil && c.Value == "1" {
			return ""
		}
		return a.devUser
	}
	return ""
}

// ResolveWithRefresh is Resolve plus a transparent token refresh: if the access
// token is missing/expired but a valid refresh_token cookie is present, it calls
// the auth service's /refresh, relays the rotated cookies to the browser, and
// validates the new access token — so the session renews without a re-login.
func (a *Auth) ResolveWithRefresh(w http.ResponseWriter, r *http.Request) string {
	if uid := a.Resolve(r); uid != "" {
		return uid
	}
	access, setCookies := a.refresh(r)
	if access == "" {
		return ""
	}
	for _, c := range setCookies {
		w.Header().Add("Set-Cookie", c) // rotated access + refresh, on the shared domain
	}
	return a.validate(r.Context(), access)
}

// refresh POSTs the refresh_token cookie to the auth service and returns the new
// access token plus the Set-Cookie headers to relay back to the browser.
func (a *Auth) refresh(r *http.Request) (string, []string) {
	if a.refreshURL == "" {
		return "", nil
	}
	rc, err := r.Cookie("refresh_token")
	if err != nil || rc.Value == "" {
		return "", nil
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, a.refreshURL, nil)
	if err != nil {
		return "", nil
	}
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rc.Value})
	resp, err := a.client.Do(req)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return "", nil
	}
	return out.AccessToken, resp.Header.Values("Set-Cookie")
}

func bearer(r *http.Request) string {
	if c, err := r.Cookie("access_token"); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func (a *Auth) validate(ctx context.Context, token string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.validateURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var out struct {
		UserID string `json:"user_id"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return ""
	}
	return out.UserID
}
