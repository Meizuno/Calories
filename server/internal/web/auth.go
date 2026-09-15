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
	// set by SharedProfileCtx so a public read can answer with the profile it
	// already resolved instead of looking it up twice
	sharedProfileKey
)

// Cookie names. Both are HttpOnly, so script on the page can never read them —
// the SPA only learns about the session through /api/session.
const (
	accessCookie  = "access_token"
	refreshCookie = "refresh_token"
	// The refresh cookie reaches the whole API, because the server renews the
	// session itself: a request arriving with an expired access token but a good
	// refresh cookie is rotated in place (ResolveWithRefresh) instead of being
	// bounced with a 401. That only works if the browser sends the cookie to the
	// route being called.
	//
	// The trade-off is deliberate: a long-lived credential now rides with every
	// API request rather than two routes. It stays HttpOnly, Secure in
	// production, SameSite=Lax and same-origin, and every use rotates it.
	refreshPath = "/api"
	// Where the refresh cookie used to live. Logout clears this path too, so a
	// browser holding a pre-change cookie is not left with a stray one.
	legacyRefreshPath = "/api/auth"
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

// Middleware gates protected routes. An expired access token is renewed in
// place rather than rejected, so a 401 now means the session is genuinely over.
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

// ResolveWithRefresh is Resolve plus automatic renewal: when the access token is
// missing or expired but the refresh cookie is still good, it rotates the pair
// and sets the new cookies, so the caller never sees the expiry.
//
// Rotation is a write, so this must only be used where a Set-Cookie can still
// be emitted — never after the response has started.
func (a *Auth) ResolveWithRefresh(w http.ResponseWriter, r *http.Request) string {
	if uid := a.Resolve(r); uid != "" {
		return uid
	}
	rc, err := r.Cookie(refreshCookie)
	if err != nil || rc.Value == "" {
		return ""
	}
	user, next, err := a.svc.Rotate(r.Context(), rc.Value, r.UserAgent())
	if err != nil {
		// Expired, revoked or replayed — drop the dead cookies so the browser
		// stops presenting them on every subsequent request.
		a.ClearSession(w)
		return ""
	}
	if err := a.SetSession(w, user.ID, next); err != nil {
		return ""
	}
	return user.ID
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
		{refreshCookie, legacyRefreshPath},
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
