package service_test

import (
	"context"
	"errors"
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

	mealID, err := diary.CreateMeal(ctx, pid, src, "Breakfast", "with honey", []service.EntryInput{
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

// An id that names nothing this profile owns must be reported, not shrugged at.
// These used to succeed silently: a scoped UPDATE or DELETE that matches no row
// is not an error to the database, so the API answered 200 and the caller had
// no way to tell the write had not happened.
func TestMissingRowsAreReported(t *testing.T) {
	diary, auth, profiles, _ := newDiary(t)
	ctx := context.Background()
	mine := profileFor(t, auth, profiles, "owner@example.com")
	theirs := profileFor(t, auth, profiles, "someone-else@example.com")

	date := day("2026-04-01")
	mealID, err := diary.CreateMeal(ctx, mine, date, "Lunch", "", []service.EntryInput{
		{Name: "Soup", Unit: "ml", Quantity: 300, Kcal: 120},
	})
	if err != nil {
		t.Fatalf("seed meal: %v", err)
	}
	view, err := diary.GetDayView(ctx, mine, date)
	if err != nil {
		t.Fatalf("read day: %v", err)
	}
	entryID := view.Meals[0].Entries[0].ID

	cases := []struct {
		name string
		run  func() error
	}{
		{"update a meal that does not exist", func() error {
			return diary.UpdateMeal(ctx, mine, 9_000_001, "X", "")
		}},
		{"delete a meal that does not exist", func() error {
			return diary.DeleteMeal(ctx, mine, 9_000_001)
		}},
		{"update an entry that does not exist", func() error {
			return diary.UpdateEntry(ctx, mine, 9_000_001, "X", "g", 1, 0, 0, 0, 0)
		}},
		{"delete an entry that does not exist", func() error {
			return diary.DeleteEntry(ctx, mine, 9_000_001)
		}},
		// Someone else's row is reported the same way as one that is not there.
		// Distinguishing them is how an id becomes enumerable.
		{"update someone else's meal", func() error {
			return diary.UpdateMeal(ctx, theirs, mealID, "X", "")
		}},
		{"delete someone else's meal", func() error {
			return diary.DeleteMeal(ctx, theirs, mealID)
		}},
		{"update someone else's entry", func() error {
			return diary.UpdateEntry(ctx, theirs, entryID, "X", "g", 1, 0, 0, 0, 0)
		}},
		{"delete someone else's entry", func() error {
			return diary.DeleteEntry(ctx, theirs, entryID)
		}},
		{"add an entry to someone else's meal", func() error {
			return diary.AddAdhocEntry(ctx, theirs, mealID, "X", "g", 1, 0, 0, 0, 0)
		}},
		{"copy someone else's meal", func() error {
			_, err := diary.CopyMeal(ctx, theirs, mealID, date)
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run(); !errors.Is(err, service.ErrNotFound) {
				t.Errorf("err = %v, want service.ErrNotFound", err)
			}
		})
	}

	// And none of that touched the real meal.
	after, err := diary.GetDayView(ctx, mine, date)
	if err != nil {
		t.Fatalf("re-read day: %v", err)
	}
	if len(after.Meals) != 1 || after.Meals[0].Meal.Name != "Lunch" || len(after.Meals[0].Entries) != 1 {
		t.Fatalf("the owner's meal was disturbed: %+v", after.Meals)
	}
}
