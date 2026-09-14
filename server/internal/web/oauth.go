package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Google is the Authorization-Code sign-in flow. The app is a confidential
// client (the secret lives on the server), so no PKCE is required — CSRF is
// handled by the `state` value, which is echoed in a short-lived cookie and
// compared on the way back.
type Google struct {
	cfg    *oauth2.Config
	client *http.Client
	secure bool
}

const (
	oauthStateCookie = "oauth_state"
	// Where to land after a successful sign-in, stashed alongside the state so a
	// deep link survives the round trip to Google.
	oauthReturnCookie = "oauth_return"
	userinfoURL       = "https://www.googleapis.com/oauth2/v3/userinfo"
)

// NewGoogle returns nil when Google sign-in is not configured, which is how the
// rest of the server tests whether to offer it.
func NewGoogle(clientID, clientSecret, redirectURL string, secure bool) *Google {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	return &Google{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
		client: &http.Client{Timeout: 10 * time.Second},
		secure: secure,
	}
}

// Start sends the browser to Google's consent screen.
func (g *Google) Start(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "could not start sign-in", http.StatusInternalServerError)
		return
	}
	g.setTemp(w, oauthStateCookie, state)
	g.setTemp(w, oauthReturnCookie, returnPath(r))
	// AccessTypeOnline: we never act on the user's behalf offline, so there is no
	// Google refresh token to store.
	http.Redirect(w, r, g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

// GoogleUser is the subset of the userinfo response the app needs.
type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// Sentinel OAuth failures. oauthCode turns each into the error code the SPA
// translates; the text here is for logs only.
var (
	errOAuthState      = errors.New("sign-in expired or was tampered with")
	errOAuthCancelled  = errors.New("google sign-in was cancelled")
	errOAuthRejected   = errors.New("google rejected the sign-in")
	errOAuthUnverified = errors.New("google account has no verified email address")
)

// oauthCode maps an Exchange failure to a stable client-side code.
func oauthCode(err error) string {
	switch {
	case errors.Is(err, errOAuthState):
		return "oauth_state"
	case errors.Is(err, errOAuthCancelled):
		return "oauth_cancelled"
	case errors.Is(err, errOAuthUnverified):
		return "oauth_unverified"
	default:
		return "oauth_failed"
	}
}

// Exchange validates the callback, swaps the code for a token and reads the
// profile. It returns where the browser should land afterwards alongside the user.
func (g *Google) Exchange(w http.ResponseWriter, r *http.Request) (GoogleUser, string, error) {
	back := "/"
	if c, err := r.Cookie(oauthReturnCookie); err == nil && strings.HasPrefix(c.Value, "/") {
		back = c.Value
	}
	g.clearTemp(w, oauthStateCookie)
	g.clearTemp(w, oauthReturnCookie)

	state, err := r.Cookie(oauthStateCookie)
	if err != nil || state.Value == "" || state.Value != r.URL.Query().Get("state") {
		return GoogleUser{}, back, errOAuthState
	}
	if e := r.URL.Query().Get("error"); e != "" {
		return GoogleUser{}, back, errOAuthCancelled
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		return GoogleUser{}, back, errOAuthState
	}

	tok, err := g.cfg.Exchange(r.Context(), code)
	if err != nil {
		return GoogleUser{}, back, errOAuthRejected
	}
	u, err := g.userinfo(r.Context(), tok)
	if err != nil {
		return GoogleUser{}, back, err
	}
	// Accounts are linked by email, so an unverified one would let a Google user
	// claim someone else's account.
	if u.Sub == "" || u.Email == "" || !u.EmailVerified {
		return GoogleUser{}, back, errOAuthUnverified
	}
	return u, back, nil
}

// userinfo reads the profile straight from Google over TLS with the freshly
// issued access token — no id_token signature verification needed, because the
// response comes from Google directly rather than through the browser.
func (g *Google) userinfo(ctx context.Context, tok *oauth2.Token) (GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := g.client.Do(req)
	if err != nil {
		return GoogleUser{}, errOAuthRejected
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GoogleUser{}, errOAuthRejected
	}
	var u GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return GoogleUser{}, errOAuthRejected
	}
	return u, nil
}

// setTemp writes a 10-minute cookie that only needs to survive the redirect to
// Google and back.
func (g *Google) setTemp(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/api/auth",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   g.secure,
		// Lax, not Strict: the callback arrives as a cross-site navigation from
		// google.com, and Strict would withhold the cookie.
		SameSite: http.SameSiteLaxMode,
	})
}

func (g *Google) clearTemp(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/api/auth", MaxAge: -1,
		HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteLaxMode,
	})
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
