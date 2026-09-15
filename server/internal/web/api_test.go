package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Meizuno/calories/internal/service"
)

func TestParseDate(t *testing.T) {
	t.Run("reads a calendar day", func(t *testing.T) {
		got, err := parseDate("2026-03-09")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got.Format("2006-01-02") != "2026-03-09" {
			t.Errorf("got %s", got.Format("2006-01-02"))
		}
		if h, m, s := got.Clock(); h|m|s != 0 {
			t.Errorf("not midnight: %v", got)
		}
	})

	t.Run("empty means today", func(t *testing.T) {
		if _, err := parseDate(""); err != nil {
			t.Fatalf("empty should be allowed: %v", err)
		}
	})

	// The whole point of the change: these used to fall back to today, so a
	// mistyped date wrote to the wrong day and said nothing about it.
	for _, in := range []string{"nonsense", "2026-13-45", "09-03-2026", "2026-3-9", "2026-03-09T10:00:00Z", " "} {
		t.Run("rejects "+in, func(t *testing.T) {
			if _, err := parseDate(in); !errors.Is(err, errBadDate) {
				t.Errorf("parseDate(%q) error = %v, want errBadDate", in, err)
			}
		})
	}
}

func TestCleanEntries(t *testing.T) {
	got := cleanEntries([]entryInput{
		{Name: "  Oats  ", Quantity: 80, Unit: " g ", Kcal: 300, Carb: 52, Protein: 10, Fat: 6},
		{Name: "", Quantity: 5},                  // no name
		{Name: "Ghost", Quantity: 0},             // no quantity
		{Name: "Negative", Quantity: 1, Fat: -3}, // macros clamp, row still counts
	})
	if len(got) != 2 {
		t.Fatalf("kept %d rows, want 2: %+v", len(got), got)
	}
	if got[0].Name != "Oats" || got[0].Unit != "g" {
		t.Errorf("not trimmed: %q %q", got[0].Name, got[0].Unit)
	}
	if got[1].Fat != 0 {
		t.Errorf("negative macro = %v, want clamped to 0", got[1].Fat)
	}
}

func TestStatsRange(t *testing.T) {
	rng := func(q string) (string, string, error) {
		r := httptest.NewRequest(http.MethodGet, "/stats?"+q, nil)
		from, to, err := statsRange(r)
		if err != nil {
			return "", "", err
		}
		return from.Format("2006-01-02"), to.Format("2006-01-02"), nil
	}

	t.Run("reversed bounds are swapped", func(t *testing.T) {
		from, to, err := rng("from=2026-03-09&to=2026-03-02")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if from != "2026-03-02" || to != "2026-03-09" {
			t.Errorf("got %s..%s", from, to)
		}
	})

	t.Run("the window is capped at a year", func(t *testing.T) {
		from, to, err := rng("from=2000-01-01&to=2026-03-09")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if from != "2025-03-09" || to != "2026-03-09" {
			t.Errorf("got %s..%s, want a one-year window", from, to)
		}
	})

	t.Run("a bad bound is an error, not a silent today", func(t *testing.T) {
		if _, _, err := rng("from=nope&to=2026-03-09"); !errors.Is(err, errBadDate) {
			t.Errorf("err = %v, want errBadDate", err)
		}
	})
}

// apiError is the API's user-facing contract for failures: the SPA translates
// the code, so these mappings are what a person ends up reading.
func TestAPIError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing row", service.ErrNotFound, http.StatusNotFound, "not_found"},
		{"wrapped missing row", errors.Join(errors.New("ctx"), service.ErrNotFound), http.StatusNotFound, "not_found"},
		{"bad date", errBadDate, http.StatusBadRequest, "invalid_date"},
		{"anything else", errors.New("pq: relation \"meals\" does not exist"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			apiError(w, httptest.NewRequest(http.MethodGet, "/", nil), c.err)

			if w.Code != c.status {
				t.Errorf("status = %d, want %d", w.Code, c.status)
			}
			var body struct{ Code, Message string }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not the {code,message} shape: %q", w.Body.String())
			}
			if body.Code != c.code {
				t.Errorf("code = %q, want %q", body.Code, c.code)
			}
			// Driver text names tables, constraints and sometimes values. None of
			// it may reach a client.
			if body.Message == c.err.Error() && c.code == "internal" {
				t.Errorf("leaked the underlying error: %q", body.Message)
			}
		})
	}
}
