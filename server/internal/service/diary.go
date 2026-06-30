package service

import (
	"context"
	"time"

	"github.com/Meizuno/calories/internal/domain"
	"github.com/Meizuno/calories/internal/store/db"
)

// View structs returned to the web layer. Totals are derived, never stored.
type MealView struct {
	Meal    db.Meal
	Entries []db.Entry
	Total   domain.Macros
}

type DayView struct {
	Date      time.Time
	Meals     []MealView
	Target    domain.Macros
	Eaten     domain.Macros
	Remaining domain.Macros
}

type Diary struct {
	q *db.Queries
}

func NewDiary(q *db.Queries) *Diary { return &Diary{q: q} }

// GetDayView assembles the meals + entries for a (profile, date). There is no day
// row; the target is the profile's current goal.
func (s *Diary) GetDayView(ctx context.Context, profileID int64, date time.Time) (DayView, error) {
	meals, err := s.q.ListMealsForDay(ctx, db.ListMealsForDayParams{ProfileID: profileID, Date: date})
	if err != nil {
		return DayView{}, err
	}
	entries, err := s.q.ListEntriesForDay(ctx, db.ListEntriesForDayParams{ProfileID: profileID, Date: date})
	if err != nil {
		return DayView{}, err
	}

	byMeal := map[int64][]db.Entry{}
	for _, e := range entries {
		byMeal[e.MealID] = append(byMeal[e.MealID], e)
	}

	var eaten domain.Macros
	mvs := make([]MealView, 0, len(meals))
	for _, m := range meals {
		es := byMeal[m.ID]
		total := entriesTotal(es)
		eaten = eaten.Add(total)
		mvs = append(mvs, MealView{Meal: m, Entries: es, Total: total})
	}

	target := s.goal(ctx, profileID)
	return DayView{
		Date:      date,
		Meals:     mvs,
		Target:    target,
		Eaten:     eaten,
		Remaining: domain.Remaining(target, eaten),
	}, nil
}

func (s *Diary) AddMeal(ctx context.Context, profileID int64, date time.Time, name, note string) error {
	pos, err := s.q.MaxMealPosition(ctx, db.MaxMealPositionParams{ProfileID: profileID, Date: date})
	if err != nil {
		return err
	}
	_, err = s.q.CreateMeal(ctx, db.CreateMealParams{ProfileID: profileID, Date: date, Name: name, Position: pos + 1, Note: strPtr(note)})
	return err
}

// EntryInput is one ad-hoc line for LogMeal; macros are taken as given (the
// caller clamps them).
type EntryInput struct {
	Name, Unit                         string
	Quantity, Kcal, Carb, Protein, Fat float64
}

// LogMeal creates a meal (with optional note) and all of its ad-hoc entries in a
// single call — the shape a chat assistant logs through the PAT API. Blank-named
// or non-positive-quantity entries are skipped. Returns the new meal id.
func (s *Diary) LogMeal(ctx context.Context, profileID int64, date time.Time, name, note string, entries []EntryInput) (int64, error) {
	pos, err := s.q.MaxMealPosition(ctx, db.MaxMealPositionParams{ProfileID: profileID, Date: date})
	if err != nil {
		return 0, err
	}
	meal, err := s.q.CreateMeal(ctx, db.CreateMealParams{ProfileID: profileID, Date: date, Name: name, Position: pos + 1, Note: strPtr(note)})
	if err != nil {
		return 0, err
	}
	for i, e := range entries {
		if e.Name == "" || e.Quantity <= 0 {
			continue
		}
		unit := e.Unit
		if unit == "" {
			unit = "g"
		}
		if _, err := s.q.CreateEntry(ctx, db.CreateEntryParams{
			MealID:   meal.ID,
			FoodID:   nil,
			Name:     e.Name,
			Quantity: e.Quantity,
			Unit:     unit,
			Position: int32(i + 1),
			Kcal:     e.Kcal,
			Carb:     e.Carb,
			Protein:  e.Protein,
			Fat:      e.Fat,
		}); err != nil {
			return meal.ID, err
		}
	}
	return meal.ID, nil
}

// ListDays returns the distinct dates that have at least one logged entry — used
// by the client to mark which calendar days are navigable.
func (s *Diary) ListDays(ctx context.Context, profileID int64) ([]time.Time, error) {
	return s.q.ListMealDays(ctx, profileID)
}

func (s *Diary) DeleteMeal(ctx context.Context, profileID, mealID int64) error {
	return s.q.DeleteMeal(ctx, db.DeleteMealParams{ID: mealID, ProfileID: profileID})
}

func (s *Diary) UpdateMeal(ctx context.Context, profileID, mealID int64, name, note string) error {
	return s.q.UpdateMeal(ctx, db.UpdateMealParams{ID: mealID, ProfileID: profileID, Name: name, Note: strPtr(note)})
}

// strPtr maps an empty string to NULL (no note) and otherwise to a pointer.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// AddFoodEntry scales the food by quantity and SNAPSHOTS the macros onto the
// entry. The meal is verified to belong to the profile (no IDOR).
func (s *Diary) AddFoodEntry(ctx context.Context, profileID, mealID, foodID int64, quantity float64) error {
	if _, err := s.q.GetMealForProfile(ctx, db.GetMealForProfileParams{ID: mealID, ProfileID: profileID}); err != nil {
		return err
	}
	food, err := s.q.GetFood(ctx, db.GetFoodParams{ID: foodID, ProfileID: profileID})
	if err != nil {
		return err
	}
	m := domain.Scale(domain.Macros{Kcal: food.Kcal, Carb: food.Carb, Protein: food.Protein, Fat: food.Fat}, food.BasisAmount, quantity)
	pos, err := s.q.MaxEntryPosition(ctx, mealID)
	if err != nil {
		return err
	}
	fid := foodID
	_, err = s.q.CreateEntry(ctx, db.CreateEntryParams{
		MealID:   mealID,
		FoodID:   &fid,
		Name:     food.Name,
		Quantity: quantity,
		Unit:     food.BasisUnit,
		Position: pos + 1,
		Kcal:     m.Kcal,
		Carb:     m.Carb,
		Protein:  m.Protein,
		Fat:      m.Fat,
	})
	return err
}

// AddAdhocEntry logs a free-typed line: name + quantity (+ optional macros),
// with no catalog food behind it. The meal is verified to belong to the profile.
func (s *Diary) AddAdhocEntry(ctx context.Context, profileID, mealID int64, name, unit string, quantity, kcal, carb, protein, fat float64) error {
	if _, err := s.q.GetMealForProfile(ctx, db.GetMealForProfileParams{ID: mealID, ProfileID: profileID}); err != nil {
		return err
	}
	if unit == "" {
		unit = "g"
	}
	pos, err := s.q.MaxEntryPosition(ctx, mealID)
	if err != nil {
		return err
	}
	_, err = s.q.CreateEntry(ctx, db.CreateEntryParams{
		MealID:   mealID,
		FoodID:   nil,
		Name:     name,
		Quantity: quantity,
		Unit:     unit,
		Position: pos + 1,
		Kcal:     kcal,
		Carb:     carb,
		Protein:  protein,
		Fat:      fat,
	})
	return err
}

func (s *Diary) DeleteEntry(ctx context.Context, profileID, entryID int64) error {
	return s.q.DeleteEntry(ctx, db.DeleteEntryParams{ID: entryID, ProfileID: profileID})
}

// UpdateEntry edits a line in place (name + quantity + macros). Ownership is
// enforced via the meal's profile in the query (no IDOR).
func (s *Diary) UpdateEntry(ctx context.Context, profileID, entryID int64, name, unit string, quantity, kcal, carb, protein, fat float64) error {
	if unit == "" {
		unit = "g"
	}
	return s.q.UpdateEntry(ctx, db.UpdateEntryParams{
		ID:        entryID,
		ProfileID: profileID,
		Name:      name,
		Quantity:  quantity,
		Unit:      unit,
		Kcal:      kcal,
		Carb:      carb,
		Protein:   protein,
		Fat:       fat,
	})
}

// goal reads the daily macro target from the profile, falling back to defaults
// if the profile is somehow missing.
func (s *Diary) goal(ctx context.Context, profileID int64) domain.Macros {
	p, err := s.q.GetProfile(ctx, profileID)
	if err != nil {
		return domain.Macros{Kcal: 2300, Carb: 253, Protein: 169, Fat: 68}
	}
	return domain.Macros{Kcal: p.Kcal, Carb: p.Carb, Protein: p.Protein, Fat: p.Fat}
}

func entriesTotal(es []db.Entry) domain.Macros {
	var t domain.Macros
	for _, e := range es {
		t = t.Add(domain.Macros{Kcal: e.Kcal, Carb: e.Carb, Protein: e.Protein, Fat: e.Fat})
	}
	return t
}
