package service

import (
	"context"

	"github.com/Meizuno/calories/internal/store/db"
)

// Profiles maps an external (SSO) user to a local profile and manages it.
// Everything else in the app hangs off the profile id.
type Profiles struct {
	q *db.Queries
}

func NewProfiles(q *db.Queries) *Profiles { return &Profiles{q: q} }

// Ensure returns the profile for the external user, creating it on first sight.
func (p *Profiles) Ensure(ctx context.Context, userID string) (db.Profile, error) {
	return p.q.EnsureProfile(ctx, userID)
}

func (p *Profiles) Get(ctx context.Context, profileID int64) (db.Profile, error) {
	return p.q.GetProfile(ctx, profileID)
}

// GetShared returns a profile by its public sharing id, but only if it is shared.
func (p *Profiles) GetShared(ctx context.Context, publicID string) (db.Profile, error) {
	return p.q.GetSharedProfile(ctx, publicID)
}

// Save persists the profile form (name + goal + sharing) and marks it onboarded.
func (p *Profiles) Save(ctx context.Context, profileID int64, name string, kcal, carb, protein, fat float64, shared bool) (db.Profile, error) {
	return p.q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID:      profileID,
		Name:    name,
		Kcal:    kcal,
		Carb:    carb,
		Protein: protein,
		Fat:     fat,
		Shared:  shared,
	})
}
