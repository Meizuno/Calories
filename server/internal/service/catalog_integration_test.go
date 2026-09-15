package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store/db"
)

// Remembering a food is an upsert against a unique index, so it only means
// anything against a real database. Skips with the rest unless
// TEST_DATABASE_URL is set.

func newCatalog(t *testing.T) (*service.Catalog, *service.Auth, *service.Profiles) {
	t.Helper()
	st := testStore(t)
	return service.NewCatalog(st.Queries),
		service.NewAuth(st.Queries, "test-secret-at-least-32-characters-long", time.Minute, time.Hour),
		service.NewProfiles(st.Queries)
}

// only returns the single food a profile knows, failing if there is not exactly
// one — which is also how the "no duplicates" expectation is enforced.
func only(t *testing.T, c *service.Catalog, pid int64) db.Food {
	t.Helper()
	foods, err := c.List(context.Background(), pid)
	if err != nil {
		t.Fatalf("list foods: %v", err)
	}
	if len(foods) != 1 {
		t.Fatalf("profile knows %d foods, want 1: %+v", len(foods), foods)
	}
	return foods[0]
}

func near(a, b float64) bool { return a-b < 0.001 && b-a < 0.001 }

func TestRememberFood(t *testing.T) {
	catalog, auth, profiles := newCatalog(t)
	ctx := context.Background()
	pid := profileFor(t, auth, profiles, "catalog@example.com")

	t.Run("stores mass per 100 of its unit", func(t *testing.T) {
		// 80 g at 300 kcal is 375 kcal per 100 g.
		if err := catalog.Remember(ctx, pid, "Oats", "g", 80, 300, 52, 10, 6); err != nil {
			t.Fatalf("remember: %v", err)
		}
		f := only(t, catalog, pid)
		if f.BasisAmount != 100 || f.BasisUnit != "g" {
			t.Fatalf("basis = %v %s, want 100 g", f.BasisAmount, f.BasisUnit)
		}
		if !near(f.Kcal, 375) || !near(f.Carb, 65) || !near(f.Protein, 12.5) || !near(f.Fat, 7.5) {
			t.Errorf("macros = %v/%v/%v/%v, want 375/65/12.5/7.5", f.Kcal, f.Carb, f.Protein, f.Fat)
		}
	})

	t.Run("re-logging corrects it in place, without a second row", func(t *testing.T) {
		// Same food, same unit, a corrected reading: 320 kcal per 80 g -> 400.
		if err := catalog.Remember(ctx, pid, "Oats", "g", 80, 320, 52, 10, 6); err != nil {
			t.Fatalf("remember again: %v", err)
		}
		if f := only(t, catalog, pid); !near(f.Kcal, 400) {
			t.Errorf("kcal = %v, want 400 after the correction", f.Kcal)
		}
	})

	t.Run("matches case-insensitively", func(t *testing.T) {
		// "oats" is the same food as "Oats" to whoever is typing it.
		if err := catalog.Remember(ctx, pid, "oats", "g", 100, 410, 52, 10, 6); err != nil {
			t.Fatalf("remember lowercase: %v", err)
		}
		f := only(t, catalog, pid)
		if !near(f.Kcal, 410) {
			t.Errorf("kcal = %v, want 410", f.Kcal)
		}
		if f.Name != "oats" {
			t.Errorf("name = %q, want the spelling most recently used", f.Name)
		}
	})

	t.Run("counts are stored per one, not per hundred", func(t *testing.T) {
		pid := profileFor(t, auth, profiles, "counts@example.com")
		// Two bananas at 214 kcal is 107 kcal each. "100 ks" would be nonsense.
		if err := catalog.Remember(ctx, pid, "Banana", "ks", 2, 214, 50, 2.6, 0.6); err != nil {
			t.Fatalf("remember: %v", err)
		}
		f := only(t, catalog, pid)
		if f.BasisAmount != 1 || f.BasisUnit != "ks" {
			t.Fatalf("basis = %v %s, want 1 ks", f.BasisAmount, f.BasisUnit)
		}
		if !near(f.Kcal, 107) {
			t.Errorf("kcal = %v, want 107", f.Kcal)
		}
	})

	t.Run("the same name in a different unit is a different food", func(t *testing.T) {
		pid := profileFor(t, auth, profiles, "units@example.com")
		if err := catalog.Remember(ctx, pid, "Milk", "ml", 200, 96, 9.6, 6.6, 3.4); err != nil {
			t.Fatalf("remember ml: %v", err)
		}
		if err := catalog.Remember(ctx, pid, "Milk", "g", 200, 96, 9.6, 6.6, 3.4); err != nil {
			t.Fatalf("remember g: %v", err)
		}
		foods, err := catalog.List(ctx, pid)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(foods) != 2 {
			t.Fatalf("got %d rows, want ml and g kept apart", len(foods))
		}
	})

	t.Run("a line with no macros teaches nothing", func(t *testing.T) {
		pid := profileFor(t, auth, profiles, "empty@example.com")
		// Someone logging "1 apple" with the fields blank must not overwrite a
		// real apple with zeroes, nor invent one.
		if err := catalog.Remember(ctx, pid, "Apple", "ks", 1, 0, 0, 0, 0); err != nil {
			t.Fatalf("remember: %v", err)
		}
		foods, err := catalog.List(ctx, pid)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(foods) != 0 {
			t.Fatalf("remembered %d foods from a macro-less line, want 0", len(foods))
		}
	})

	t.Run("ignores a nameless or zero-quantity line", func(t *testing.T) {
		pid := profileFor(t, auth, profiles, "junk@example.com")
		if err := catalog.Remember(ctx, pid, "   ", "g", 100, 200, 1, 1, 1); err != nil {
			t.Fatalf("blank name: %v", err)
		}
		// A zero quantity would divide by zero on the way to the basis.
		if err := catalog.Remember(ctx, pid, "Ghost", "g", 0, 200, 1, 1, 1); err != nil {
			t.Fatalf("zero quantity: %v", err)
		}
		foods, err := catalog.List(ctx, pid)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(foods) != 0 {
			t.Fatalf("remembered %d junk rows, want 0", len(foods))
		}
	})

	t.Run("one profile never sees another's foods", func(t *testing.T) {
		mine := profileFor(t, auth, profiles, "mine@example.com")
		theirs := profileFor(t, auth, profiles, "theirs@example.com")
		if err := catalog.Remember(ctx, mine, "Secret", "g", 100, 123, 1, 1, 1); err != nil {
			t.Fatalf("remember: %v", err)
		}
		foods, err := catalog.List(ctx, theirs)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(foods) != 0 {
			t.Fatalf("leaked %d foods across profiles", len(foods))
		}
	})
}
