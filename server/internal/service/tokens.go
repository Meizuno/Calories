package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/store/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// PATPrefix marks our personal access tokens so a PAT can be told apart from a
// session access token before any DB work.
const PATPrefix = "cal_pat_"

// Scopes a PAT may hold. A real session is full access; a PAT is limited to
// these. Anything destructive/structural carries no scope and is full-only.
var validScopes = map[string]bool{"read": true, "add": true}

// ErrTokenNotFound is returned by Revoke when no active token matched.
var ErrTokenNotFound = errors.New("token not found")

type Tokens struct {
	q *db.Queries
}

func NewTokens(q *db.Queries) *Tokens { return &Tokens{q: q} }

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return PATPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// CleanScopes keeps only known, de-duplicated scopes.
func CleanScopes(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		if validScopes[s] && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

// Create mints a token for the profile. The raw value is returned ONCE; only its
// sha256 hash is stored.
func (t *Tokens) Create(ctx context.Context, profileID int64, name string, scopes []string, expiresAt *time.Time) (string, db.PersonalAccessToken, error) {
	raw, err := generateRawToken()
	if err != nil {
		return "", db.PersonalAccessToken{}, err
	}
	var exp pgtype.Timestamptz
	if expiresAt != nil {
		exp = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	row, err := t.q.CreatePat(ctx, db.CreatePatParams{
		ProfileID: profileID,
		Name:      name,
		TokenHash: hashToken(raw),
		Scopes:    CleanScopes(scopes),
		ExpiresAt: exp,
	})
	return raw, row, err
}

func (t *Tokens) List(ctx context.Context, profileID int64) ([]db.PersonalAccessToken, error) {
	return t.q.ListPats(ctx, profileID)
}

func (t *Tokens) Revoke(ctx context.Context, profileID, id int64) error {
	n, err := t.q.RevokePat(ctx, db.RevokePatParams{ID: id, ProfileID: profileID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// Resolve maps a raw PAT to its profile + granted scopes (and bumps last_used_at).
// ok is false when the token is the wrong shape, unknown, revoked, or expired.
func (t *Tokens) Resolve(ctx context.Context, raw string) (profileID int64, scopes []string, ok bool) {
	if !strings.HasPrefix(raw, PATPrefix) {
		return 0, nil, false
	}
	row, err := t.q.GetPatByHash(ctx, hashToken(raw))
	if err != nil {
		return 0, nil, false
	}
	_ = t.q.TouchPat(ctx, row.ID)
	return row.ProfileID, CleanScopes(row.Scopes), true
}
