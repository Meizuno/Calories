package web

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/domain"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store/db"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handlers struct {
	diary    *service.Diary
	catalog  *service.Catalog
	profiles *service.Profiles
	tokens   *service.Tokens
	auth     *Auth
	authsvc  *service.Auth
	// google is nil when Google sign-in is not configured; /api/session reports
	// that so the SPA knows whether to show the button.
	google *Google
	// allowedEmails gates who may complete the Google flow (lowercased).
	allowedEmails map[string]bool
	// allowRegistration opens POST /api/auth/register; off by default.
	allowRegistration bool
}

func NewHandlers(diary *service.Diary, catalog *service.Catalog, profiles *service.Profiles, tokens *service.Tokens, auth *Auth, authsvc *service.Auth, google *Google, allowedEmails []string, allowRegistration bool) *Handlers {
	allowed := make(map[string]bool, len(allowedEmails))
	for _, e := range allowedEmails {
		allowed[strings.ToLower(strings.TrimSpace(e))] = true
	}
	return &Handlers{
		diary: diary, catalog: catalog, profiles: profiles, tokens: tokens,
		auth: auth, authsvc: authsvc, google: google,
		allowedEmails: allowed, allowRegistration: allowRegistration,
	}
}

// ── DTOs ────────────────────────────────────────────────────────────────────

type macros struct {
	Kcal    float64 `json:"kcal"`
	Carb    float64 `json:"carb"`
	Protein float64 `json:"protein"`
	Fat     float64 `json:"fat"`
}

type entryResp struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Kcal     float64 `json:"kcal"`
	Carb     float64 `json:"carb"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
}

type mealResp struct {
	ID      int64       `json:"id"`
	Name    string      `json:"name"`
	Note    string      `json:"note"`
	Entries []entryResp `json:"entries"`
	Total   macros      `json:"total"`
}

type dayResp struct {
	Date      string     `json:"date"`
	Target    macros     `json:"target"`
	Eaten     macros     `json:"eaten"`
	Remaining macros     `json:"remaining"`
	Meals     []mealResp `json:"meals"`
}

type foodResp struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	BasisUnit   string  `json:"basisUnit"`
	BasisAmount float64 `json:"basisAmount"`
	Kcal        float64 `json:"kcal"`
	Carb        float64 `json:"carb"`
	Protein     float64 `json:"protein"`
	Fat         float64 `json:"fat"`
}

func mac(m domain.Macros) macros { return macros{m.Kcal, m.Carb, m.Protein, m.Fat} }

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dayDTO(dv service.DayView) dayResp {
	meals := make([]mealResp, 0, len(dv.Meals))
	for _, mv := range dv.Meals {
		entries := make([]entryResp, 0, len(mv.Entries))
		for _, e := range mv.Entries {
			entries = append(entries, entryResp{e.ID, e.Name, e.Quantity, e.Unit, e.Kcal, e.Carb, e.Protein, e.Fat})
		}
		meals = append(meals, mealResp{ID: mv.Meal.ID, Name: mv.Meal.Name, Note: deref(mv.Meal.Note), Entries: entries, Total: mac(mv.Total)})
	}
	return dayResp{
		Date:      dv.Date.Format("2006-01-02"),
		Target:    mac(dv.Target),
		Eaten:     mac(dv.Eaten),
		Remaining: mac(dv.Remaining),
		Meals:     meals,
	}
}

// ── Profile + session endpoints ──────────────────────────────────────────────

type profileResp struct {
	PublicID  string `json:"publicId"`
	Name      string `json:"name"`
	Shared    bool   `json:"shared"`
	Onboarded bool   `json:"onboarded"`
	Goal      macros `json:"goal"`
}

func profileDTO(p db.Profile) profileResp {
	return profileResp{
		PublicID:  p.PublicID,
		Name:      p.Name,
		Shared:    p.Shared,
		Onboarded: p.Onboarded,
		Goal:      macros{p.Kcal, p.Carb, p.Protein, p.Fat},
	}
}

// Session is public: it reports whether the caller has a session and, if so, their
// account and profile. The SPA uses it to bootstrap (welcome vs app, onboarding)
// and to learn which sign-in methods this deployment offers.
func (h *Handlers) Session(w http.ResponseWriter, r *http.Request) {
	// Renews before answering: without this a page opened after the access token
	// expired would report "not authenticated" and bounce a still-valid session
	// to the sign-in screen.
	uid := h.auth.ResolveWithRefresh(w, r)
	if uid == "" {
		writeJSON(w, map[string]any{"session": map[string]any{
			"authenticated": false,
			"google":        h.google != nil,
			"registration":  h.allowRegistration,
		}})
		return
	}
	h.writeSession(w, r, uid)
}

// writeSession renders the authenticated session payload. Shared by /api/session
// and every endpoint that establishes one, so the SPA always gets one shape.
func (h *Handlers) writeSession(w http.ResponseWriter, r *http.Request, userID string) {
	user, err := h.authsvc.GetUser(r.Context(), userID)
	if err != nil {
		apiError(w, r, err)
		return
	}
	prof, err := h.profiles.Ensure(r.Context(), userID)
	if err != nil {
		apiError(w, r, err)
		return
	}
	providers, err := h.authsvc.Providers(r.Context(), userID)
	if err != nil {
		apiError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"session": map[string]any{
		"authenticated": true,
		"google":        h.google != nil,
		"registration":  h.allowRegistration,
		"user": map[string]any{
			"email":       user.Email,
			"name":        user.Name,
			"hasPassword": user.PasswordHash != nil,
			"providers":   providers,
		},
		"profile": profileDTO(prof),
	}})
}

func (h *Handlers) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	prof, err := h.profiles.Get(r.Context(), ProfileID(r.Context()))
	if err != nil {
		apiError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"profile": profileDTO(prof)})
}

func (h *Handlers) SaveProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string  `json:"name"`
		Kcal    float64 `json:"kcal"`
		Carb    float64 `json:"carb"`
		Protein float64 `json:"protein"`
		Fat     float64 `json:"fat"`
		Shared  bool    `json:"shared"`
	}
	if !decode(w, r, &req) {
		return
	}
	prof, err := h.profiles.Save(r.Context(), ProfileID(r.Context()), strings.TrimSpace(req.Name),
		nonNeg(req.Kcal), nonNeg(req.Carb), nonNeg(req.Protein), nonNeg(req.Fat), req.Shared)
	if err != nil {
		apiError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"profile": profileDTO(prof)})
}

// ── Personal access tokens (full-session only) ───────────────────────────────

type patResp struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	LastUsedAt *string  `json:"lastUsedAt"`
	ExpiresAt  *string  `json:"expiresAt"`
}

func tsPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}

func patDTO(p db.PersonalAccessToken) patResp {
	scopes := p.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return patResp{
		ID:         p.ID,
		Name:       p.Name,
		Scopes:     scopes,
		CreatedAt:  p.CreatedAt.UTC().Format(time.RFC3339),
		LastUsedAt: tsPtr(p.LastUsedAt),
		ExpiresAt:  tsPtr(p.ExpiresAt),
	}
}

func (h *Handlers) listPatsJSON(w http.ResponseWriter, r *http.Request, profileID int64) {
	rows, err := h.tokens.List(r.Context(), profileID)
	if err != nil {
		apiError(w, r, err)
		return
	}
	out := make([]patResp, 0, len(rows))
	for _, p := range rows {
		out = append(out, patDTO(p))
	}
	writeJSON(w, map[string]any{"pats": out})
}

func (h *Handlers) ListPats(w http.ResponseWriter, r *http.Request) {
	h.listPatsJSON(w, r, ProfileID(r.Context()))
}

func (h *Handlers) CreatePat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresAt string   `json:"expiresAt"`
	}
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	scopes := service.CleanScopes(req.Scopes)
	if name == "" || len(scopes) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_token_request", "a token needs a name and at least one scope")
		return
	}
	var exp *time.Time
	if req.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ExpiresAt); err == nil {
			exp = &t
		}
	}
	raw, row, err := h.tokens.Create(r.Context(), ProfileID(r.Context()), name, scopes, exp)
	if err != nil {
		apiError(w, r, err)
		return
	}
	// The raw token is returned exactly once, here.
	writeJSON(w, map[string]any{"token": raw, "pat": patDTO(row)})
}

func (h *Handlers) RevokePat(w http.ResponseWriter, r *http.Request) {
	pid := ProfileID(r.Context())
	if err := h.tokens.Revoke(r.Context(), pid, idParam(r)); err != nil && !errors.Is(err, service.ErrTokenNotFound) {
		apiError(w, r, err)
		return
	}
	h.listPatsJSON(w, r, pid)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// writeError replies with a machine-readable `code` alongside the English
// message. The SPA translates the code, so the server stays language-agnostic
// and rewording a message never changes what a user reads.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

// drain reads whatever is left of the request body and throws it away.
//
// Rejecting a request before anything reads its body — the Gate turning away an
// expired session, a decoder giving up halfway — leaves bytes in flight. Go
// then closes the connection rather than finishing the exchange, and the caller
// sees a reset instead of the 401 or 400 that was actually written. Draining
// first lets the response land.
func drain(r *http.Request) {
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		drain(r)
		writeError(w, http.StatusBadRequest, "invalid_body", "the request body is not valid JSON")
		return false
	}
	return true
}

func idParam(r *http.Request) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return n
}

// parseDate reads a YYYY-MM-DD day. An empty value means today, which is the
// useful default for "just show me now". Anything else that will not parse is
// an error: it used to fall back to today as well, so a typo in a date wrote to
// the wrong day and said nothing.
//
// Days are UTC calendar dates. The server cannot know the caller's timezone, so
// clients send the date they mean rather than relying on this.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, errBadDate
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

var errBadDate = errors.New("date must be YYYY-MM-DD")

func badDate(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_date", errBadDate.Error())
}

// nonNeg clamps user-supplied macros to >= 0 (defence in depth; clients guard too).
func nonNeg(f float64) float64 {
	if f < 0 {
		return 0
	}
	return f
}

// apiError turns a service error into the one response shape the API uses. An
// id that names nothing this caller owns is a 404 whether it does not exist or
// merely is not theirs — saying which would let anyone enumerate other people's
// rows. Anything unrecognised is logged here and reported as a bare 500: pgx
// error text names tables, constraints and sometimes values, and none of that
// belongs in a response.
func apiError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no such item")
	case errors.Is(err, errBadDate):
		badDate(w)
	default:
		logRequestError(r, "unhandled", err)
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}

func logRequestError(r *http.Request, what string, err error) {
	slog.Error("api error", "what", what, "method", r.Method, "path", r.URL.Path, "err", err)
}
