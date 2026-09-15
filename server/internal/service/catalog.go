// Package service holds the application logic (orchestration over the store).
package service

import (
	"context"
	"strings"

	"github.com/Meizuno/calories/internal/store/db"
)

type Catalog struct {
	q *db.Queries
}

func NewCatalog(q *db.Queries) *Catalog { return &Catalog{q: q} }

func (c *Catalog) List(ctx context.Context, profileID int64) ([]db.Food, error) {
	return c.q.ListFoods(ctx, profileID)
}

func (c *Catalog) Create(ctx context.Context, profileID int64, name, unit string, basis, kcal, carb, protein, fat float64) error {
	_, err := c.q.CreateFood(ctx, db.CreateFoodParams{
		ProfileID:   profileID,
		Name:        name,
		BasisUnit:   unit,
		BasisAmount: basis,
		Kcal:        kcal,
		Carb:        carb,
		Protein:     protein,
		Fat:         fat,
	})
	return err
}

// basisFor is the amount a food's macros are remembered per. Mass and volume
// are conventionally quoted per 100; a count has no sensible 100 of it ("100 ks"
// of banana), so those are remembered per one.
func basisFor(unit string) float64 {
	switch unit {
	case "g", "ml":
		return 100
	default:
		return 1
	}
}

// Remember records what a logged line taught us about a food: its macros scaled
// to a standard basis, so any future quantity can be derived from them. Called
// whenever an entry is logged or corrected, so the catalogue builds itself
// instead of asking anyone to curate one.
//
// A line with no macros teaches nothing, and is skipped rather than written: a
// hurried "1 apple" with the fields left blank must not overwrite a real apple
// with zeroes.
func (c *Catalog) Remember(ctx context.Context, profileID int64, name, unit string, quantity, kcal, carb, protein, fat float64) error {
	name = strings.TrimSpace(name)
	if name == "" || quantity <= 0 {
		return nil
	}
	if kcal <= 0 && carb <= 0 && protein <= 0 && fat <= 0 {
		return nil
	}
	if unit == "" {
		unit = "g"
	}
	basis := basisFor(unit)
	per := basis / quantity
	return c.q.UpsertFood(ctx, db.UpsertFoodParams{
		ProfileID:   profileID,
		Name:        name,
		BasisUnit:   unit,
		BasisAmount: basis,
		Kcal:        kcal * per,
		Carb:        carb * per,
		Protein:     protein * per,
		Fat:         fat * per,
	})
}

func (c *Catalog) Delete(ctx context.Context, profileID, id int64) error {
	return notFound(c.q.DeleteFood(ctx, db.DeleteFoodParams{ID: id, ProfileID: profileID}))
}
