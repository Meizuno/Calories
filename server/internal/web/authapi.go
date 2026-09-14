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
//
// Failures carry a stable `code` (see writeError) that the SPA translates, so
// the API stays language-agnostic and rewording a message never changes what a
// user reads.

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
		writeError(w, http.StatusForbidden, "registration_closed", "registration is closed")
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
		writeError(w, http.StatusUnauthorized, "session_expired", "no refresh token")
		return
	}
	user, next, err := h.authsvc.Rotate(r.Context(), rc.Value, r.UserAgent())
	if err != nil {
		// Expired, revoked or replayed — drop the dead cookies so the browser
		// stops presenting them.
		h.auth.ClearSession(w)
		writeError(w, http.StatusUnauthorized, "session_expired", "refresh token rejected")
		return
	}
	if err := h.auth.SetSession(w, user.ID, next); err != nil {
		writeError(w, http.StatusInternalServerError, "unknown", "could not start session")
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
		writeError(w, http.StatusNotFound, "google_unavailable", "google sign-in is not configured")
		return
	}
	h.google.Start(w, r)
}

// GoogleCallback completes the flow and lands the browser back in the SPA. A
// failure is carried as an error CODE in the query string, because the user
// arrives here by navigation: there is no fetch waiting to read a JSON body.
func (h *Handlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.google == nil {
		writeError(w, http.StatusNotFound, "google_unavailable", "google sign-in is not configured")
		return
	}
	gu, back, err := h.google.Exchange(w, r)
	if err != nil {
		h.failGoogle(w, r, oauthCode(err))
		return
	}
	// Allowlist: only the configured address(es) may sign in. Checked before the
	// account is touched, so a rejected sign-in creates nothing.
	if !h.allowedEmails[strings.ToLower(strings.TrimSpace(gu.Email))] {
		h.failGoogle(w, r, "not_allowed")
		return
	}
	user, err := h.authsvc.UpsertGoogle(r.Context(), gu.Sub, gu.Email, gu.Name)
	if err != nil {
		h.failGoogle(w, r, "unknown")
		return
	}
	refresh, err := h.authsvc.IssueRefresh(r.Context(), user.ID, "", r.UserAgent())
	if err == nil {
		err = h.auth.SetSession(w, user.ID, refresh)
	}
	if err != nil {
		h.failGoogle(w, r, "unknown")
		return
	}
	if _, err := h.profiles.Ensure(r.Context(), user.ID); err != nil {
		h.failGoogle(w, r, "unknown")
		return
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// failGoogle sends the browser back to the sign-in page carrying an error code
// for the SPA to translate.
func (h *Handlers) failGoogle(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(code), http.StatusFound)
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
		writeError(w, http.StatusInternalServerError, "unknown", "could not load account")
		return
	}
	if user.PasswordHash != nil {
		if _, err := h.authsvc.Login(r.Context(), user.Email, req.Current); err != nil {
			writeError(w, http.StatusForbidden, "wrong_password", "current password is incorrect")
			return
		}
	}
	if err := h.authsvc.SetPassword(r.Context(), uid, req.New); err != nil {
		authError(w, err)
		return
	}
	if err := h.authsvc.RevokeAll(r.Context(), uid); err != nil {
		writeError(w, http.StatusInternalServerError, "unknown", "could not revoke sessions")
		return
	}
	h.startSession(w, r, uid)
}

// startSession issues a fresh token pair, makes sure the profile exists, and
// replies with the same shape as /api/session so the SPA can adopt it directly.
func (h *Handlers) startSession(w http.ResponseWriter, r *http.Request, userID string) {
	refresh, err := h.authsvc.IssueRefresh(r.Context(), userID, "", r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unknown", "could not start session")
		return
	}
	if err := h.auth.SetSession(w, userID, refresh); err != nil {
		writeError(w, http.StatusInternalServerError, "unknown", "could not start session")
		return
	}
	h.writeSession(w, r, userID)
}

// authError maps the sentinel errors from the service onto a status plus a
// stable code. Anything unrecognised stays a 500 rather than leaking its text.
func authError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBadCredentials):
		writeError(w, http.StatusUnauthorized, "bad_credentials", err.Error())
	case errors.Is(err, service.ErrNoPassword):
		writeError(w, http.StatusUnauthorized, "no_password", err.Error())
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, service.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
	case errors.Is(err, service.ErrLongPassword):
		writeError(w, http.StatusBadRequest, "long_password", err.Error())
	case errors.Is(err, service.ErrBadEmail):
		writeError(w, http.StatusBadRequest, "bad_email", err.Error())
	case errors.Is(err, service.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "session_expired", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unknown", "something went wrong")
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
