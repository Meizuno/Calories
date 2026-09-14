package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Meizuno/calories/internal/service"
)

// Auth endpoints. Sign-up and sign-in issue the cookie pair; /refresh rotates it;
// /logout revokes the refresh family and clears both cookies.

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// Register is closed unless ALLOW_REGISTRATION is set. The route stays mounted
// so a client gets a clear "closed" message instead of a 404 that reads like a
// bug. Note the gate lives here, not in the service: cmd/seed creates the dev
// account through service.Auth directly and must keep working.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	if !h.allowRegistration {
		http.Error(w, "registration is closed", http.StatusForbidden)
		return
	}
	var req credentials
	if !decode(w, r, &req) {
		return
	}
	user, err := h.authsvc.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		authError(w, err)
		return
	}
	h.startSession(w, r, user.ID)
}

func (h *Handlers) LoginPassword(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if !decode(w, r, &req) {
		return
	}
	user, err := h.authsvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		authError(w, err)
		return
	}
	h.startSession(w, r, user.ID)
}

// Refresh rotates the session. It is the only endpoint the refresh cookie is sent
// to; the SPA calls it once on a 401 and then retries the original request.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	rc, err := r.Cookie(refreshCookie)
	if err != nil || rc.Value == "" {
		h.auth.ClearSession(w)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, next, err := h.authsvc.Rotate(r.Context(), rc.Value, r.UserAgent())
	if err != nil {
		// Expired, revoked or replayed — drop the dead cookies so the browser
		// stops presenting them.
		h.auth.ClearSession(w)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.auth.SetSession(w, user.ID, next); err != nil {
		http.Error(w, "could not start session", http.StatusInternalServerError)
		return
	}
	h.writeSession(w, r, user.ID)
}

// Logout revokes the whole rotation family (so a stolen copy of the refresh token
// dies with it) and clears the cookies.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if rc, err := r.Cookie(refreshCookie); err == nil && rc.Value != "" {
		h.authsvc.RevokeRefresh(r.Context(), rc.Value)
	}
	h.auth.ClearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

// GoogleStart redirects to Google. It is a top-level navigation, not a fetch —
// an OAuth consent screen cannot be rendered inside an XHR.
func (h *Handlers) GoogleStart(w http.ResponseWriter, r *http.Request) {
	if h.google == nil {
		http.Error(w, "google sign-in is not configured", http.StatusNotFound)
		return
	}
	h.google.Start(w, r)
}

// GoogleCallback completes the flow and lands the browser back in the SPA. Errors
// are carried as a query parameter because the user arrives here by navigation:
// there is no fetch waiting to read a JSON body.
func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.google == nil {
		http.Error(w, "google sign-in is not configured", http.StatusNotFound)
		return
	}
	gu, back, err := h.google.Exchange(w, r)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	// Allowlist: only the configured address(es) may sign in. Checked before the
	// account is touched, so a rejected sign-in creates nothing.
	if !h.allowedEmails[strings.ToLower(strings.TrimSpace(gu.Email))] {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("this account is not allowed to sign in"), http.StatusFound)
		return
	}
	user, err := h.authsvc.UpsertGoogle(r.Context(), gu.Sub, gu.Email, gu.Name)
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("could not complete sign-in"), http.StatusFound)
		return
	}
	refresh, err := h.authsvc.IssueRefresh(r.Context(), user.ID, "", r.UserAgent())
	if err == nil {
		err = h.auth.SetSession(w, user.ID, refresh)
	}
	if err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("could not start session"), http.StatusFound)
		return
	}
	if _, err := h.profiles.Ensure(r.Context(), user.ID); err != nil {
		http.Redirect(w, r, "/login?error="+url.QueryEscape("could not load your profile"), http.StatusFound)
		return
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// ChangePassword sets or replaces the password. Setting one on a Google-only
// account needs no current password; replacing an existing one does. Either way
// every other session is signed out, then this one is re-established.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !decode(w, r, &req) {
		return
	}
	uid := UserID(r.Context())
	user, err := h.authsvc.GetUser(r.Context(), uid)
	if err != nil {
		http.Error(w, "could not load account", http.StatusInternalServerError)
		return
	}
	if user.PasswordHash != nil {
		if _, err := h.authsvc.Login(r.Context(), user.Email, req.Current); err != nil {
			http.Error(w, "current password is incorrect", http.StatusForbidden)
			return
		}
	}
	if err := h.authsvc.SetPassword(r.Context(), uid, req.New); err != nil {
		authError(w, err)
		return
	}
	if err := h.authsvc.RevokeAll(r.Context(), uid); err != nil {
		http.Error(w, "could not revoke sessions", http.StatusInternalServerError)
		return
	}
	h.startSession(w, r, uid)
}

// startSession issues a fresh token pair, makes sure the profile exists, and
// replies with the same shape as /api/session so the SPA can adopt it directly.
func (h *Handlers) startSession(w http.ResponseWriter, r *http.Request, userID string) {
	refresh, err := h.authsvc.IssueRefresh(r.Context(), userID, "", r.UserAgent())
	if err != nil {
		http.Error(w, "could not start session", http.StatusInternalServerError)
		return
	}
	if err := h.auth.SetSession(w, userID, refresh); err != nil {
		http.Error(w, "could not start session", http.StatusInternalServerError)
		return
	}
	h.writeSession(w, r, userID)
}

// authError maps the service's sentinel errors onto status codes, keeping
// anything unrecognised a 500 rather than leaking its text.
func authError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBadCredentials), errors.Is(err, service.ErrNoPassword):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, service.ErrEmailTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, service.ErrWeakPassword), errors.Is(err, service.ErrLongPassword), errors.Is(err, service.ErrBadEmail):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrInvalidToken):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	default:
		http.Error(w, "something went wrong", http.StatusInternalServerError)
	}
}

// returnPath extracts a SAFE local return path from ?return= (must be a path on
// this origin — never an absolute or scheme-relative URL, to avoid open redirects).
func returnPath(r *http.Request) string {
	p := r.URL.Query().Get("return")
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return "/"
	}
	return p
}
