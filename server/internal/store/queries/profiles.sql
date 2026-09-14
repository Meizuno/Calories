-- name: GetProfile :one
SELECT * FROM profiles WHERE id = $1;

-- name: GetSharedProfile :one
SELECT * FROM profiles WHERE public_id = $1 AND shared = true;

-- name: EnsureProfile :one
INSERT INTO profiles (user_id) VALUES ($1)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles
SET name = $2, kcal = $3, carb = $4, protein = $5, fat = $6, shared = $7, onboarded = true, updated_at = now()
WHERE id = $1
RETURNING *;

-- Unclaimed profiles are pre-cutover rows: they still carry the external SSO id
-- in legacy_user_id but no local user. `cmd/claim` lists and attaches them.

-- name: ListUnclaimedProfiles :many
SELECT * FROM profiles
WHERE user_id IS NULL AND legacy_user_id IS NOT NULL
ORDER BY created_at;

-- name: ClaimProfile :one
UPDATE profiles SET user_id = $2, updated_at = now()
WHERE id = $1 AND user_id IS NULL
RETURNING *;

-- name: DeleteProfile :exec
DELETE FROM profiles WHERE id = $1;
