package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/domain"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store/db"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	diary        *service.Diary
	catalog      *service.Catalog
	profiles     *service.Profiles
	auth         *Auth
	loginURL     string
	logoutURL    string
	cookieDomain string
	httpClient   *http.Client
}

func NewHandlers(diary *service.Diary, catalog *service.Catalog, profiles *service.Profiles, auth *Auth, loginURL, logoutURL, cookieDomain string) *Handlers {
	return &Handlers{
		diary: diary, catalog: catalog, profiles: profiles, auth: auth,
		loginURL: loginURL, logoutURL: logoutURL, cookieDomain: cookieDomain,
		httpClient: &http.Client{Timeout: 10 * time.Second},
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

// ── Diary endpoints (mutations return the refreshed day) ─────────────────────

func (h *Handlers) GetDay(w http.ResponseWriter, r *http.Request) {
	h.respondDay(w, r, ProfileID(r.Context()), parseDate(r.URL.Query().Get("date")))
}

func (h *Handlers) AddMeal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
		Name string `json:"name"`
		Note string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	pid, date := ProfileID(r.Context()), parseDate(req.Date)
	if req.Name != "" {
		_ = h.diary.AddMeal(r.Context(), pid, date, req.Name, strings.TrimSpace(req.Note))
	}
	h.respondDay(w, r, pid, date)
}

func (h *Handlers) UpdateMeal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
		Name string `json:"name"`
		Note string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	pid := ProfileID(r.Context())
	if name := strings.TrimSpace(req.Name); name != "" {
		_ = h.diary.UpdateMeal(r.Context(), pid, idParam(r), name, strings.TrimSpace(req.Note))
	}
	h.respondDay(w, r, pid, parseDate(req.Date))
}

func (h *Handlers) DeleteMeal(w http.ResponseWriter, r *http.Request) {
	pid := ProfileID(r.Context())
	_ = h.diary.DeleteMeal(r.Context(), pid, idParam(r))
	h.respondDay(w, r, pid, parseDate(r.URL.Query().Get("date")))
}

func (h *Handlers) AddEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date     string  `json:"date"`
		MealID   int64   `json:"mealId"`
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
		Kcal     float64 `json:"kcal"`
		Carb     float64 `json:"carb"`
		Protein  float64 `json:"protein"`
		Fat      float64 `json:"fat"`
	}
	if !decode(w, r, &req) {
		return
	}
	pid, date := ProfileID(r.Context()), parseDate(req.Date)
	name := strings.TrimSpace(req.Name)
	if req.MealID > 0 && name != "" && req.Quantity > 0 {
		_ = h.diary.AddAdhocEntry(r.Context(), pid, req.MealID, name, req.Unit,
			req.Quantity, nonNeg(req.Kcal), nonNeg(req.Carb), nonNeg(req.Protein), nonNeg(req.Fat))
	}
	h.respondDay(w, r, pid, date)
}

// nonNeg clamps user-supplied macros to >= 0 (defence in depth; the client also guards).
func nonNeg(f float64) float64 {
	if f < 0 {
		return 0
	}
	return f
}

func (h *Handlers) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date     string  `json:"date"`
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
		Kcal     float64 `json:"kcal"`
		Carb     float64 `json:"carb"`
		Protein  float64 `json:"protein"`
		Fat      float64 `json:"fat"`
	}
	if !decode(w, r, &req) {
		return
	}
	pid := ProfileID(r.Context())
	if name := strings.TrimSpace(req.Name); name != "" && req.Quantity > 0 {
		_ = h.diary.UpdateEntry(r.Context(), pid, idParam(r), name, req.Unit,
			req.Quantity, nonNeg(req.Kcal), nonNeg(req.Carb), nonNeg(req.Protein), nonNeg(req.Fat))
	}
	h.respondDay(w, r, pid, parseDate(req.Date))
}

func (h *Handlers) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	pid := ProfileID(r.Context())
	_ = h.diary.DeleteEntry(r.Context(), pid, idParam(r))
	h.respondDay(w, r, pid, parseDate(r.URL.Query().Get("date")))
}

func (h *Handlers) respondDay(w http.ResponseWriter, r *http.Request, profileID int64, date time.Time) {
	dv, err := h.diary.GetDayView(r.Context(), profileID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dayDTO(dv))
}

// ── Catalog endpoints ────────────────────────────────────────────────────────

func (h *Handlers) ListFoods(w http.ResponseWriter, r *http.Request) {
	h.respondFoods(w, r, ProfileID(r.Context()))
}

func (h *Handlers) CreateFood(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		BasisUnit   string  `json:"basisUnit"`
		BasisAmount float64 `json:"basisAmount"`
		Kcal        float64 `json:"kcal"`
		Carb        float64 `json:"carb"`
		Protein     float64 `json:"protein"`
		Fat         float64 `json:"fat"`
	}
	if !decode(w, r, &req) {
		return
	}
	pid := ProfileID(r.Context())
	if req.BasisUnit == "" {
		req.BasisUnit = "g"
	}
	if req.BasisAmount <= 0 {
		req.BasisAmount = 100
	}
	if req.Name != "" {
		_ = h.catalog.Create(r.Context(), pid, req.Name, req.BasisUnit, req.BasisAmount, req.Kcal, req.Carb, req.Protein, req.Fat)
	}
	h.respondFoods(w, r, pid)
}

func (h *Handlers) DeleteFood(w http.ResponseWriter, r *http.Request) {
	pid := ProfileID(r.Context())
	_ = h.catalog.Delete(r.Context(), pid, idParam(r))
	h.respondFoods(w, r, pid)
}

func (h *Handlers) respondFoods(w http.ResponseWriter, r *http.Request, profileID int64) {
	foods, err := h.catalog.List(r.Context(), profileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]foodResp, 0, len(foods))
	for _, f := range foods {
		out = append(out, foodResp{f.ID, f.Name, f.BasisUnit, f.BasisAmount, f.Kcal, f.Carb, f.Protein, f.Fat})
	}
	writeJSON(w, out)
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
// profile. The SPA uses it to bootstrap (welcome vs app, onboarding, login link).
func (h *Handlers) Session(w http.ResponseWriter, r *http.Request) {
	uid := h.auth.ResolveWithRefresh(w, r)
	if uid == "" {
		writeJSON(w, map[string]any{"authenticated": false})
		return
	}
	prof, err := h.profiles.Ensure(r.Context(), uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"authenticated": true,
		"profile":       profileDTO(prof),
	})
}

// Login (public) starts sign-in: it redirects the browser to the central auth
// entrypoint with this app's return URL as redirect_url. The SPA reaches it via a
// top-level navigation (the OAuth flow inherently needs one — a fetch can't do it).
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	back := returnPath(r)
	if h.loginURL == "" {
		// No central auth configured (local dev) — clear any simulated logout and go
		// back; the dev user is "logged in" again.
		http.SetCookie(w, &http.Cookie{Name: "dev_logout", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	u, err := url.Parse(h.loginURL)
	if err != nil {
		http.Error(w, "bad login url", http.StatusInternalServerError)
		return
	}
	q := u.Query()
	// Always return to the app ROOT: the auth allowlist matches redirect_url by
	// exact full URL, so a single entry (https://<host>/) covers every login —
	// regardless of which page triggered it.
	q.Set("redirect_url", absoluteURL(r, "/"))
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// returnPath extracts a SAFE local return path from ?return= (must be a path on
// this origin — never an absolute/scheme-relative URL, to avoid open redirects).
func returnPath(r *http.Request) string {
	p := r.URL.Query().Get("return")
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return "/"
	}
	return p
}

func absoluteURL(r *http.Request, path string) string {
	scheme := "https"
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	} else if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host + path
}

// Logout (public) revokes the refresh token at the central auth — best-effort,
// forwarding the refresh cookie server-side — then clears the session cookies on
// the shared parent domain. Proxied here because the central /logout is POST-only
// with SameSite=Lax cookies and no CORS, so the SPA can't call it cross-origin.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if h.logoutURL != "" {
		if rc, err := r.Cookie("refresh_token"); err == nil && rc.Value != "" {
			if req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.logoutURL, nil); err == nil {
				req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rc.Value})
				if resp, err := h.httpClient.Do(req); err == nil {
					_ = resp.Body.Close()
				}
			}
		}
	}
	h.clearCookie(w, "access_token")
	h.clearCookie(w, "refresh_token")
	if h.loginURL == "" {
		// Local dev: no real cookie to clear, so record a simulated logout that the
		// dev fallback (auth.go) honours until the next /api/login.
		http.SetCookie(w, &http.Cookie{Name: "dev_logout", Value: "1", Path: "/", MaxAge: 86400, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handlers) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	prof, err := h.profiles.Get(r.Context(), ProfileID(r.Context()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, profileDTO(prof))
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, profileDTO(prof))
}

// SharedProfile / SharedDay are public, read-only views of a profile that has
// opted into sharing (shared = true), addressed by its public uuid.
func (h *Handlers) SharedProfile(w http.ResponseWriter, r *http.Request) {
	prof, err := h.profiles.GetShared(r.Context(), chi.URLParam(r, "uuid"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, profileDTO(prof))
}

func (h *Handlers) SharedDay(w http.ResponseWriter, r *http.Request) {
	prof, err := h.profiles.GetShared(r.Context(), chi.URLParam(r, "uuid"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	dv, err := h.diary.GetDayView(r.Context(), prof.ID, parseDate(r.URL.Query().Get("date")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dayDTO(dv))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

func idParam(r *http.Request) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return n
}

func parseDate(s string) time.Time {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
