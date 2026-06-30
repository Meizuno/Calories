// Package service holds the application logic (orchestration over the store).
package service

import (
	"context"

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

func (c *Catalog) Delete(ctx context.Context, profileID, id int64) error {
	return c.q.DeleteFood(ctx, db.DeleteFoodParams{ID: id, ProfileID: profileID})
}
