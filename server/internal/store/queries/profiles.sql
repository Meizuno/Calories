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
