package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
)

// Copying a meal forward is the "same breakfast most mornings" path. It is all
// database work -- read a meal, write a new one -- so it is only meaningful
// against a real database. Skips with the rest unless TEST_DATABASE_URL is set.

func newDiary(t *testing.T) (*service.Diary, *service.Auth, *service.Profiles, *store.Store) {
	t.Helper()
	st := testStore(t)
	return service.NewDiary(st.Queries),
		service.NewAuth(st.Queries, "test-secret-at-least-32-characters-long", time.Minute, time.Hour),
		service.NewProfiles(st.Queries),
		st
}

// profileFor registers an account and returns the profile id everything else
// hangs off.
func profileFor(t *testing.T, a *service.Auth, p *service.Profiles, email string) int64 {
	t.Helper()
	u := mustRegister(t, a, email, "correcthorse")
	prof, err := p.Ensure(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("ensure profile for %s: %v", email, err)
	}
	return prof.ID
}

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestCopyMeal(t *testing.T) {
	diary, auth, profiles, _ := newDiary(t)
	ctx := context.Background()
	pid := profileFor(t, auth, profiles, "copy@example.com")

	src := day("2026-03-02")
	dst := day("2026-03-09")

	mealID, err := diary.LogMeal(ctx, pid, src, "Breakfast", "with honey", []service.EntryInput{
		{Name: "Oats", Unit: "g", Quantity: 80, Kcal: 300, Carb: 52, Protein: 10, Fat: 6},
		{Name: "Milk", Unit: "ml", Quantity: 200, Kcal: 96, Carb: 9.6, Protein: 6.6, Fat: 3.4},
	})
	if err != nil {
		t.Fatalf("log source meal: %v", err)
	}

	t.Run("copies the name, the note and every entry", func(t *testing.T) {
		newID, err := diary.CopyMeal(ctx, pid, mealID, dst)
		if err != nil {
			t.Fatalf("copy: %v", err)
		}
		if newID == mealID {
			t.Fatal("copy reused the source meal row")
		}

		from, err := diary.GetDayView(ctx, pid, src)
		if err != nil {
			t.Fatalf("read source day: %v", err)
		}
		to, err := diary.GetDayView(ctx, pid, dst)
		if err != nil {
			t.Fatalf("read target day: %v", err)
		}
		if len(to.Meals) != 1 {
			t.Fatalf("target day has %d meals, want 1", len(to.Meals))
		}

		orig, dup := from.Meals[0], to.Meals[0]
		if dup.Meal.Name != orig.Meal.Name {
			t.Errorf("name = %q, want %q", dup.Meal.Name, orig.Meal.Name)
		}
		if deref(dup.Meal.Note) != "with honey" {
			t.Errorf("note = %q, want %q", deref(dup.Meal.Note), "with honey")
		}
		if len(dup.Entries) != len(orig.Entries) {
			t.Fatalf("copied %d entries, want %d", len(dup.Entries), len(orig.Entries))
		}
		for i := range orig.Entries {
			a, b := orig.Entries[i], dup.Entries[i]
			if a.ID == b.ID {
				t.Errorf("entry %d reused the source row", i)
			}
			if b.Name != a.Name || b.Quantity != a.Quantity || b.Unit != a.Unit {
				t.Errorf("entry %d = %q %v%s, want %q %v%s", i, b.Name, b.Quantity, b.Unit, a.Name, a.Quantity, a.Unit)
			}
			if b.Kcal != a.Kcal || b.Carb != a.Carb || b.Protein != a.Protein || b.Fat != a.Fat {
				t.Errorf("entry %d macros = %v/%v/%v/%v, want %v/%v/%v/%v",
					i, b.Kcal, b.Carb, b.Protein, b.Fat, a.Kcal, a.Carb, a.Protein, a.Fat)
			}
		}
		// The source is untouched: this is a copy, not a move.
		if orig.Total.Kcal != dup.Total.Kcal {
			t.Errorf("totals diverged: source %v, copy %v", orig.Total.Kcal, dup.Total.Kcal)
		}
	})

	t.Run("appends after the meals already on the target day", func(t *testing.T) {
		// The previous subtest already put one copy on dst; a second must land
		// after it rather than fighting for the same position.
		if _, err := diary.CopyMeal(ctx, pid, mealID, dst); err != nil {
			t.Fatalf("second copy: %v", err)
		}
		to, err := diary.GetDayView(ctx, pid, dst)
		if err != nil {
			t.Fatalf("read target day: %v", err)
		}
		if len(to.Meals) != 2 {
			t.Fatalf("target day has %d meals, want 2", len(to.Meals))
		}
		if a, b := to.Meals[0].Meal.Position, to.Meals[1].Meal.Position; a >= b {
			t.Errorf("positions not increasing: %d then %d", a, b)
		}
	})

	t.Run("will not copy another profile's meal", func(t *testing.T) {
		other := profileFor(t, auth, profiles, "stranger@example.com")
		if _, err := diary.CopyMeal(ctx, other, mealID, dst); err == nil {
			t.Fatal("copied a meal belonging to someone else")
		}
		view, err := diary.GetDayView(ctx, other, dst)
		if err != nil {
			t.Fatalf("read stranger's day: %v", err)
		}
		if len(view.Meals) != 0 {
			t.Fatalf("stranger's day gained %d meals", len(view.Meals))
		}
	})
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
