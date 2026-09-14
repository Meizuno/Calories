package service

import (
	"context"

	"github.com/Meizuno/calories/internal/store/db"
)

// Profiles maps a local user (users.id) to a profile and manages it. Everything
// else in the app hangs off the profile id.
type Profiles struct {
	q *db.Queries
}

func NewProfiles(q *db.Queries) *Profiles { return &Profiles{q: q} }

// Ensure returns the profile for the user, creating it on first sight.
func (p *Profiles) Ensure(ctx context.Context, userID string) (db.Profile, error) {
	return p.q.EnsureProfile(ctx, &userID)
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

// Unclaimed lists profiles left over from the external-SSO era: they still hold
// the old auth-service id in legacy_user_id but no local user. See cmd/claim.
func (p *Profiles) Unclaimed(ctx context.Context) ([]db.Profile, error) {
	return p.q.ListUnclaimedProfiles(ctx)
}

// Claim attaches an orphaned legacy profile to a local user, handing over its
// whole diary. The profile the user got on signup is deleted first, since a user
// may own only one.
func (p *Profiles) Claim(ctx context.Context, profileID int64, userID string) (db.Profile, error) {
	if existing, err := p.q.EnsureProfile(ctx, &userID); err == nil && existing.ID != profileID {
		if err := p.q.DeleteProfile(ctx, existing.ID); err != nil {
			return db.Profile{}, err
		}
	}
	return p.q.ClaimProfile(ctx, db.ClaimProfileParams{ID: profileID, UserID: &userID})
}
