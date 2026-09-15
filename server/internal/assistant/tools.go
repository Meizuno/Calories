package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/service"
)

// The tools the model is given. They are thin: each one parses its arguments,
// calls the same service method the HTTP API calls, and returns a small,
// self-describing object.
//
// Deliberately absent: anything that edits or deletes. Entries snapshot their
// macros precisely so that correcting a food never rewrites what a past day
// says was eaten, and there is no undo for an edit. A model that misreads an
// instruction should at worst add a meal the person can delete, never silently
// rewrite their history.
//
// Dates are YYYY-MM-DD, matching the API. "today" is resolved server-side so a
// model that cannot count days still lands on the right one.

const dateFormat = "2006-01-02"

func today() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// parseDay accepts a date or the word "today"/"yesterday", because a model
// asked "what did I eat yesterday" otherwise has to do calendar arithmetic and
// sometimes gets it wrong.
func parseDay(s string) (time.Time, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "today":
		return today(), nil
	case "yesterday":
		return today().AddDate(0, 0, -1), nil
	}
	t, err := time.Parse(dateFormat, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be YYYY-MM-DD, \"today\" or \"yesterday\"")
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

// round keeps the numbers the model sees short. Nobody needs 14 decimal places
// of protein, and every digit is a token.
func round(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

type macroDTO struct {
	Kcal    float64 `json:"kcal"`
	Carb    float64 `json:"carb"`
	Protein float64 `json:"protein"`
	Fat     float64 `json:"fat"`
}

func mac(kcal, carb, protein, fat float64) macroDTO {
	return macroDTO{round(kcal), round(carb), round(protein), round(fat)}
}

// Tools builds the full set. The caller's scopes decide which are offered.
func Tools(diary *service.Diary, catalog *service.Catalog) []Tool {
	return []Tool{
		getDay(diary),
		getStats(diary),
		searchFoods(catalog),
		logMeal(diary, catalog),
	}
}

func schema(s string) json.RawMessage { return json.RawMessage(s) }

// ── read ─────────────────────────────────────────────────────────────────────

func getDay(diary *service.Diary) Tool {
	return Tool{
		Name:  "get_day",
		Scope: "read",
		Description: "What the person ate on one day: every meal with its items, " +
			"the totals so far, their daily goal and what is left of it.",
		Schema: schema(`{
  "type": "object",
  "properties": {
    "date": { "type": "string", "description": "YYYY-MM-DD, or \"today\" / \"yesterday\". Defaults to today." }
  }
}`),
		Run: func(ctx context.Context, profileID int64, args json.RawMessage) (any, error) {
			var in struct {
				Date string `json:"date"`
			}
			_ = json.Unmarshal(args, &in)
			date, err := parseDay(in.Date)
			if err != nil {
				return map[string]string{"error": err.Error()}, nil
			}
			dv, err := diary.GetDayView(ctx, profileID, date)
			if err != nil {
				return nil, err
			}
			meals := make([]any, 0, len(dv.Meals))
			for _, m := range dv.Meals {
				items := make([]any, 0, len(m.Entries))
				for _, e := range m.Entries {
					items = append(items, map[string]any{
						"name": e.Name, "quantity": round(e.Quantity), "unit": e.Unit,
						"macros": mac(e.Kcal, e.Carb, e.Protein, e.Fat),
					})
				}
				meals = append(meals, map[string]any{
					"id": m.Meal.ID, "name": m.Meal.Name, "items": items,
					"total": mac(m.Total.Kcal, m.Total.Carb, m.Total.Protein, m.Total.Fat),
				})
			}
			return map[string]any{
				"date":      date.Format(dateFormat),
				"meals":     meals,
				"eaten":     mac(dv.Eaten.Kcal, dv.Eaten.Carb, dv.Eaten.Protein, dv.Eaten.Fat),
				"goal":      mac(dv.Target.Kcal, dv.Target.Carb, dv.Target.Protein, dv.Target.Fat),
				"remaining": mac(dv.Remaining.Kcal, dv.Remaining.Carb, dv.Remaining.Protein, dv.Remaining.Fat),
			}, nil
		},
	}
}

func getStats(diary *service.Diary) Tool {
	return Tool{
		Name:  "get_stats",
		Scope: "read",
		Description: "Daily totals across a range of days, plus the daily goal. Use this " +
			"for questions about trends, averages or how a week went.",
		Schema: schema(`{
  "type": "object",
  "required": ["from", "to"],
  "properties": {
    "from": { "type": "string", "description": "First day, YYYY-MM-DD (inclusive)." },
    "to":   { "type": "string", "description": "Last day, YYYY-MM-DD (inclusive). \"today\" is allowed." }
  }
}`),
		Run: func(ctx context.Context, profileID int64, args json.RawMessage) (any, error) {
			var in struct{ From, To string }
			_ = json.Unmarshal(args, &in)
			from, err := parseDay(in.From)
			if err != nil {
				return map[string]string{"error": err.Error()}, nil
			}
			to, err := parseDay(in.To)
			if err != nil {
				return map[string]string{"error": err.Error()}, nil
			}
			if from.After(to) {
				from, to = to, from
			}
			// Same one-year ceiling the HTTP API applies: an unbounded range is a
			// scan, and the reply would be too long to be useful anyway.
			if earliest := to.AddDate(-1, 0, 0); from.Before(earliest) {
				from = earliest
			}
			sv, err := diary.GetStats(ctx, profileID, from, to)
			if err != nil {
				return nil, err
			}
			days := make([]any, 0, len(sv.Days))
			for _, d := range sv.Days {
				days = append(days, map[string]any{
					"date":   d.Date.Format(dateFormat),
					"macros": mac(d.Totals.Kcal, d.Totals.Carb, d.Totals.Protein, d.Totals.Fat),
				})
			}
			return map[string]any{
				"from": sv.From.Format(dateFormat),
				"to":   sv.To.Format(dateFormat),
				"goal": mac(sv.Goal.Kcal, sv.Goal.Carb, sv.Goal.Protein, sv.Goal.Fat),
				// Only days with something logged are returned; say so, or the
				// model will read the gaps as zeroes and report a fasting week.
				"note": "Only days with logged entries appear. A missing day means nothing was recorded, not that nothing was eaten.",
				"days": days,
			}, nil
		},
	}
}

func searchFoods(catalog *service.Catalog) Tool {
	return Tool{
		Name:  "search_foods",
		Scope: "read",
		Description: "Foods this person has logged before, with macros per a standard " +
			"amount (per 100 g, or per 1 for counted things). Check here before " +
			"guessing a food's macros — their own record is better than an estimate.",
		Schema: schema(`{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "Part of a name. Omit to list everything known." }
  }
}`),
		Run: func(ctx context.Context, profileID int64, args json.RawMessage) (any, error) {
			var in struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal(args, &in)
			foods, err := catalog.List(ctx, profileID)
			if err != nil {
				return nil, err
			}
			q := strings.ToLower(strings.TrimSpace(in.Query))
			out := make([]any, 0, len(foods))
			for _, f := range foods {
				if q != "" && !strings.Contains(strings.ToLower(f.Name), q) {
					continue
				}
				out = append(out, map[string]any{
					"name": f.Name, "per": round(f.BasisAmount), "unit": f.BasisUnit,
					"macros": mac(f.Kcal, f.Carb, f.Protein, f.Fat),
				})
				// A long list is mostly wasted tokens, and the model can narrow.
				if len(out) >= 40 {
					break
				}
			}
			return map[string]any{"foods": out, "matched": len(out)}, nil
		},
	}
}

// ── write ────────────────────────────────────────────────────────────────────

func logMeal(diary *service.Diary, catalog *service.Catalog) Tool {
	return Tool{
		Name:  "log_meal",
		Scope: "add",
		Description: "Record a meal and everything in it. Macros are for the quantity " +
			"stated, not per 100 g. Confirm what you are about to log with the " +
			"person before calling this — it writes to their diary.",
		Schema: schema(`{
  "type": "object",
  "required": ["meal", "entries"],
  "properties": {
    "date":  { "type": "string", "description": "YYYY-MM-DD, or \"today\" / \"yesterday\". Defaults to today." },
    "meal":  { "type": "string", "description": "Name of the meal, e.g. \"Breakfast\"." },
    "note":  { "type": "string" },
    "entries": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["name", "quantity", "unit"],
        "properties": {
          "name":     { "type": "string" },
          "quantity": { "type": "number", "description": "How much, in the unit below." },
          "unit":     { "type": "string", "description": "g, ml, ks (pieces) or porce (portions)." },
          "kcal":     { "type": "number", "description": "Total for this quantity, not per 100 g." },
          "carb":     { "type": "number" },
          "protein":  { "type": "number" },
          "fat":      { "type": "number" }
        }
      }
    }
  }
}`),
		Run: func(ctx context.Context, profileID int64, args json.RawMessage) (any, error) {
			var in struct {
				Date    string `json:"date"`
				Meal    string `json:"meal"`
				Note    string `json:"note"`
				Entries []struct {
					Name     string  `json:"name"`
					Quantity float64 `json:"quantity"`
					Unit     string  `json:"unit"`
					Kcal     float64 `json:"kcal"`
					Carb     float64 `json:"carb"`
					Protein  float64 `json:"protein"`
					Fat      float64 `json:"fat"`
				} `json:"entries"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return map[string]string{"error": "arguments were not valid JSON"}, nil
			}
			name := strings.TrimSpace(in.Meal)
			if name == "" {
				return map[string]string{"error": "the meal needs a name"}, nil
			}
			date, err := parseDay(in.Date)
			if err != nil {
				return map[string]string{"error": err.Error()}, nil
			}
			if date.After(today()) {
				return map[string]string{"error": "that day is in the future"}, nil
			}

			entries := make([]service.EntryInput, 0, len(in.Entries))
			for _, e := range in.Entries {
				n := strings.TrimSpace(e.Name)
				if n == "" || e.Quantity <= 0 {
					continue
				}
				unit := strings.TrimSpace(e.Unit)
				if unit == "" {
					unit = "g"
				}
				entries = append(entries, service.EntryInput{
					Name: n, Unit: unit, Quantity: e.Quantity,
					Kcal: nonNeg(e.Kcal), Carb: nonNeg(e.Carb),
					Protein: nonNeg(e.Protein), Fat: nonNeg(e.Fat),
				})
			}
			if len(entries) == 0 {
				return map[string]string{"error": "no item had both a name and a quantity above zero"}, nil
			}

			id, err := diary.CreateMeal(ctx, profileID, date, name, strings.TrimSpace(in.Note), entries)
			if err != nil {
				return nil, err
			}
			// Logging teaches the catalogue, exactly as it does through the API.
			var total macroDTO
			for _, e := range entries {
				_ = catalog.Remember(ctx, profileID, e.Name, e.Unit, e.Quantity, e.Kcal, e.Carb, e.Protein, e.Fat)
				total.Kcal += e.Kcal
				total.Carb += e.Carb
				total.Protein += e.Protein
				total.Fat += e.Fat
			}
			return map[string]any{
				"logged":  true,
				"mealId":  id,
				"date":    date.Format(dateFormat),
				"meal":    name,
				"items":   len(entries),
				"total":   mac(total.Kcal, total.Carb, total.Protein, total.Fat),
				"confirm": "Tell the person what was logged so they can check it.",
			}, nil
		},
	}
}

func nonNeg(f float64) float64 {
	if f < 0 {
		return 0
	}
	return f
}
