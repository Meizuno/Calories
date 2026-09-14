package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/service"
)

type ctxKey int

const (
	userIDKey ctxKey = iota
	profileIDKey
	scopesKey
	fullKey
)

// Cookie names. Both are HttpOnly, so script on the page can never read them —
// the SPA only learns about the session through /api/session.
const (
	accessCookie  = "access_token"
	refreshCookie = "refresh_token"
	// The refresh cookie is confined to /api/auth, so it rides along only with
	// refresh and logout instead of every API request. The access JWT is what
	// authenticates normal traffic; when it expires the SPA calls
	// POST /api/auth/refresh once and retries.
	refreshPath = "/api/auth"
)

// UserID returns the authenticated local user id placed in the context by Auth.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// ProfileID returns the local profile id placed in the context by the Gate.
func ProfileID(ctx context.Context) int64 {
	v, _ := ctx.Value(profileIDKey).(int64)
	return v
}

// Scopes returns the granted scopes for a PAT principal (nil for a full session).
func Scopes(ctx context.Context) []string {
	v, _ := ctx.Value(scopesKey).([]string)
	return v
}

// IsFull reports whether the caller is a full session (vs a scoped PAT).
func IsFull(ctx context.Context) bool {
	v, _ := ctx.Value(fullKey).(bool)
	return v
}

// Auth resolves the current user from this app's own tokens: a signed access JWT
// in a cookie, renewed from the rotating refresh cookie at /api/auth/refresh.
// There is no external auth service.
type Auth struct {
	svc    *service.Auth
	secure bool
}

func NewAuth(svc *service.Auth, secure bool) *Auth {
	return &Auth{svc: svc, secure: secure}
}

// Middleware gates protected routes: it 401s when there is no valid access token,
// which the SPA turns into a refresh-and-retry (and then a trip to /login).
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := a.Resolve(r)
		if uid == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Resolve returns the user id carried by a valid access token, or "" if there is
// none. Read-only and allocation-cheap: signature check, no database round-trip.
func (a *Auth) Resolve(r *http.Request) string {
	tok := bearer(r)
	if tok == "" || strings.HasPrefix(tok, service.PATPrefix) {
		return ""
	}
	uid, err := a.svc.ParseAccess(tok)
	if err != nil {
		return ""
	}
	return uid
}

// SetSession writes both cookies: the short-lived access JWT and the rotating
// refresh token.
func (a *Auth) SetSession(w http.ResponseWriter, userID, refresh string) error {
	access, err := a.svc.SignAccess(userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookie,
		Value:    access,
		Path:     "/",
		MaxAge:   int(a.svc.AccessTTL().Seconds()),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    refresh,
		Path:     refreshPath,
		MaxAge:   int(a.svc.RefreshTTL().Seconds()),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSession expires both cookies. The paths must match the ones they were set
// with, or the browser keeps the originals.
func (a *Auth) ClearSession(w http.ResponseWriter) {
	for _, c := range []struct{ name, path string }{
		{accessCookie, "/"},
		{refreshCookie, refreshPath},
	} {
		http.SetCookie(w, &http.Cookie{
			Name:     c.name,
			Value:    "",
			Path:     c.path,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   a.secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// bearer pulls the session token from the cookie, or from an Authorization header
// (which is also how a PAT arrives — the caller distinguishes them by prefix).
func bearer(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if c, err := r.Cookie(accessCookie); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
