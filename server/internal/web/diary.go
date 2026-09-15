package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/service"
	"github.com/go-chi/chi/v5"
)

// Diary and catalog endpoints.
//
// Two conventions hold throughout, and both exist so the API stays usable by
// something other than our own SPA:
//
//   - Every mutation answers with the day it affected, wrapped: {"day": {...}}.
//     The envelope is the point — a response that is already an object can grow
//     a second key without breaking a client that only reads the first.
//   - Every failure answers with {"code","message"} and a real status. Nothing
//     is swallowed: a write that did not happen must never look like one that
//     did, which is exactly what returning an unchanged day used to do.

// ── reads ────────────────────────────────────────────────────────────────────
//
// These serve a signed-in owner and an anonymous visitor to a shared profile
// alike. Whoever the caller is, SharedProfileCtx or the Gate has already put the
// profile being read in the context, so there is one handler rather than a pair
// that must be kept in step.

func (h *Handlers) GetDay(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		badDate(w)
		return
	}
	h.respondDay(w, r, ProfileID(r.Context()), date)
}

// ListDays returns every date (YYYY-MM-DD) that has logged data, so a client can
// enable only those days in a calendar.
func (h *Handlers) ListDays(w http.ResponseWriter, r *http.Request) {
	days, err := h.diary.ListDays(r.Context(), ProfileID(r.Context()))
	if err != nil {
		apiError(w, r, err)
		return
	}
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Format("2006-01-02")
	}
	writeJSON(w, map[string]any{"days": out})
}

// GetStats returns per-day macro totals over ?from=&to= plus the daily goal.
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	from, to, err := statsRange(r)
	if err != nil {
		badDate(w)
		return
	}
	sv, err := h.diary.GetStats(r.Context(), ProfileID(r.Context()), from, to)
	if err != nil {
		apiError(w, r, err)
		return
	}
	days := make([]dayTotalResp, 0, len(sv.Days))
	for _, d := range sv.Days {
		days = append(days, dayTotalResp{
			Date:    d.Date.Format("2006-01-02"),
			Kcal:    d.Totals.Kcal,
			Carb:    d.Totals.Carb,
			Protein: d.Totals.Protein,
			Fat:     d.Totals.Fat,
		})
	}
	writeJSON(w, map[string]any{"stats": statsResp{
		From: sv.From.Format("2006-01-02"),
		To:   sv.To.Format("2006-01-02"),
		Goal: mac(sv.Goal),
		Days: days,
	}})
}

type dayTotalResp struct {
	Date    string  `json:"date"`
	Kcal    float64 `json:"kcal"`
	Carb    float64 `json:"carb"`
	Protein float64 `json:"protein"`
	Fat     float64 `json:"fat"`
}

type statsResp struct {
	From string         `json:"from"`
	To   string         `json:"to"`
	Goal macros         `json:"goal"`
	Days []dayTotalResp `json:"days"`
}

// statsRange parses ?from=&to= (inclusive), swaps reversed bounds, and caps the
// window at one year so a hand-crafted range cannot ask for an unbounded scan.
func statsRange(r *http.Request) (from, to time.Time, err error) {
	q := r.URL.Query()
	if from, err = parseDate(q.Get("from")); err != nil {
		return
	}
	if to, err = parseDate(q.Get("to")); err != nil {
		return
	}
	if from.After(to) {
		from, to = to, from
	}
	if earliest := to.AddDate(-1, 0, 0); from.Before(earliest) {
		from = earliest
	}
	return from, to, nil
}

// ── meals ────────────────────────────────────────────────────────────────────

// entryInput is one ad-hoc line, accepted both when creating a meal complete
// with its food and when adding a line to a meal that already exists.
type entryInput struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Kcal     float64 `json:"kcal"`
	Carb     float64 `json:"carb"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
}

// clean drops the lines that describe nothing — blank name, no quantity — and
// clamps macros to >= 0. Skipping rather than rejecting is deliberate: a client
// sending a partly-filled row means "not this one", not "fail the request".
func cleanEntries(in []entryInput) []service.EntryInput {
	out := make([]service.EntryInput, 0, len(in))
	for _, e := range in {
		name := strings.TrimSpace(e.Name)
		if name == "" || e.Quantity <= 0 {
			continue
		}
		out = append(out, service.EntryInput{
			Name:     name,
			Unit:     strings.TrimSpace(e.Unit),
			Quantity: e.Quantity,
			Kcal:     nonNeg(e.Kcal),
			Carb:     nonNeg(e.Carb),
			Protein:  nonNeg(e.Protein),
			Fat:      nonNeg(e.Fat),
		})
	}
	return out
}

// CreateMeal makes a meal, optionally complete with its entries. One endpoint
// for both callers: the diary posts a bare meal and fills it in afterwards, an
// assistant posts the whole thing at once.
func (h *Handlers) CreateMeal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
		Name string `json:"name"`
		// The unversioned /api/log alias calls this field "meal". Accepted here
		// so whatever assistant is already posting to it keeps working.
		Meal    string       `json:"meal"`
		Note    string       `json:"note"`
		Entries []entryInput `json:"entries"`
	}
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.Meal)
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, "name_required", "a meal needs a name")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		badDate(w)
		return
	}
	pid := ProfileID(r.Context())
	entries := cleanEntries(req.Entries)
	if _, err := h.diary.CreateMeal(r.Context(), pid, date, name, strings.TrimSpace(req.Note), entries); err != nil {
		apiError(w, r, err)
		return
	}
	// Entries arriving with the meal teach the catalogue too, exactly as they
	// would had they been added one at a time.
	for _, e := range entries {
		h.remember(r, pid, e)
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
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name_required", "a meal needs a name")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		badDate(w)
		return
	}
	pid := ProfileID(r.Context())
	if err := h.diary.UpdateMeal(r.Context(), pid, idParam(r), name, strings.TrimSpace(req.Note)); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondDay(w, r, pid, date)
}

func (h *Handlers) DeleteMeal(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		badDate(w)
		return
	}
	pid := ProfileID(r.Context())
	if err := h.diary.DeleteMeal(r.Context(), pid, idParam(r)); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondDay(w, r, pid, date)
}

// CopyMeal duplicates a meal onto another day, entries and all. It answers with
// the day it copied INTO, not the one it came from: that is the day that
// changed, and the only one the caller could not already see.
func (h *Handlers) CopyMeal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"` // the day to copy INTO
	}
	if !decode(w, r, &req) {
		return
	}
	to, err := parseDate(req.Date)
	if err != nil {
		badDate(w)
		return
	}
	pid := ProfileID(r.Context())
	if _, err := h.diary.CopyMeal(r.Context(), pid, idParam(r), to); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondDay(w, r, pid, to)
}

// ── entries ──────────────────────────────────────────────────────────────────

// AddEntry adds one line to the meal named in the path. The meal is an address,
// not a field: it used to travel in the body, which left a create looking
// nothing like the update and delete beside it.
func (h *Handlers) AddEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
		entryInput
	}
	if !decode(w, r, &req) {
		return
	}
	lines := cleanEntries([]entryInput{req.entryInput})
	if len(lines) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_entry", "an item needs a name and a quantity above zero")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		badDate(w)
		return
	}
	pid, e := ProfileID(r.Context()), lines[0]
	if err := h.diary.AddAdhocEntry(r.Context(), pid, idParam(r), e.Name, e.Unit,
		e.Quantity, e.Kcal, e.Carb, e.Protein, e.Fat); err != nil {
		apiError(w, r, err)
		return
	}
	h.remember(r, pid, e)
	h.respondDay(w, r, pid, date)
}

func (h *Handlers) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
		entryInput
	}
	if !decode(w, r, &req) {
		return
	}
	lines := cleanEntries([]entryInput{req.entryInput})
	if len(lines) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_entry", "an item needs a name and a quantity above zero")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		badDate(w)
		return
	}
	pid, e := ProfileID(r.Context()), lines[0]
	if err := h.diary.UpdateEntry(r.Context(), pid, idParam(r), e.Name, e.Unit,
		e.Quantity, e.Kcal, e.Carb, e.Protein, e.Fat); err != nil {
		apiError(w, r, err)
		return
	}
	h.remember(r, pid, e)
	h.respondDay(w, r, pid, date)
}

func (h *Handlers) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		badDate(w)
		return
	}
	pid := ProfileID(r.Context())
	if err := h.diary.DeleteEntry(r.Context(), pid, idParam(r)); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondDay(w, r, pid, date)
}

// remember teaches the catalogue what a logged line says about its food. Best
// effort on purpose, and the one place errors are still dropped: failing to
// learn a food must never cost someone the entry they just logged. It is logged
// rather than silently discarded.
func (h *Handlers) remember(r *http.Request, profileID int64, e service.EntryInput) {
	if err := h.catalog.Remember(r.Context(), profileID, e.Name, e.Unit,
		e.Quantity, e.Kcal, e.Carb, e.Protein, e.Fat); err != nil {
		logRequestError(r, "remember food", err)
	}
}

// ── catalog ──────────────────────────────────────────────────────────────────

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
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name_required", "a food needs a name")
		return
	}
	if req.BasisUnit == "" {
		req.BasisUnit = "g"
	}
	if req.BasisAmount <= 0 {
		req.BasisAmount = 100
	}
	pid := ProfileID(r.Context())
	if err := h.catalog.Create(r.Context(), pid, name, req.BasisUnit, req.BasisAmount,
		nonNeg(req.Kcal), nonNeg(req.Carb), nonNeg(req.Protein), nonNeg(req.Fat)); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondFoods(w, r, pid)
}

func (h *Handlers) DeleteFood(w http.ResponseWriter, r *http.Request) {
	pid := ProfileID(r.Context())
	if err := h.catalog.Delete(r.Context(), pid, idParam(r)); err != nil {
		apiError(w, r, err)
		return
	}
	h.respondFoods(w, r, pid)
}

func (h *Handlers) respondFoods(w http.ResponseWriter, r *http.Request, profileID int64) {
	foods, err := h.catalog.List(r.Context(), profileID)
	if err != nil {
		apiError(w, r, err)
		return
	}
	out := make([]foodResp, 0, len(foods))
	for _, f := range foods {
		out = append(out, foodResp{f.ID, f.Name, f.BasisUnit, f.BasisAmount, f.Kcal, f.Carb, f.Protein, f.Fat})
	}
	writeJSON(w, map[string]any{"foods": out})
}

// ── shared profiles ──────────────────────────────────────────────────────────

// SharedProfileCtx resolves {uuid} to the profile that published it and stores
// its id under the same context key the Gate uses for a signed-in caller. Every
// read handler above then serves both audiences unchanged, instead of each one
// needing a public twin that repeats the lookup and then drifts.
func (h *Handlers) SharedProfileCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prof, err := h.profiles.GetShared(r.Context(), chi.URLParam(r, "uuid"))
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "no such shared profile")
			return
		}
		ctx := context.WithValue(r.Context(), profileIDKey, prof.ID)
		ctx = context.WithValue(ctx, sharedProfileKey, profileDTO(prof))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SharedProfile returns the public face of the profile SharedProfileCtx resolved.
func (h *Handlers) SharedProfile(w http.ResponseWriter, r *http.Request) {
	prof, _ := r.Context().Value(sharedProfileKey).(profileResp)
	writeJSON(w, map[string]any{"profile": prof})
}

func (h *Handlers) respondDay(w http.ResponseWriter, r *http.Request, profileID int64, date time.Time) {
	dv, err := h.diary.GetDayView(r.Context(), profileID, date)
	if err != nil {
		apiError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"day": dayDTO(dv)})
}
