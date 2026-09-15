-- name: ListFoods :many
SELECT * FROM foods
WHERE profile_id = $1 AND archived = false
ORDER BY name;

-- name: GetFood :one
SELECT * FROM foods WHERE id = $1 AND profile_id = $2;

-- name: CreateFood :one
INSERT INTO foods (profile_id, name, basis_unit, basis_amount, kcal, carb, protein, fat)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpsertFood :exec
-- Remember a food, or correct what we already remembered about it. Macros are
-- stored per basis_amount of basis_unit (100 g, 1 ks, ...) so any quantity can
-- be scaled from them later. Last write wins: fixing a value once fixes every
-- suggestion that follows.
INSERT INTO foods (profile_id, name, basis_unit, basis_amount, kcal, carb, protein, fat)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (profile_id, lower(name), basis_unit) DO UPDATE
SET name         = EXCLUDED.name,
    basis_amount = EXCLUDED.basis_amount,
    kcal         = EXCLUDED.kcal,
    carb         = EXCLUDED.carb,
    protein      = EXCLUDED.protein,
    fat          = EXCLUDED.fat,
    updated_at   = now();

-- name: DeleteFood :execrows
DELETE FROM foods WHERE id = $1 AND profile_id = $2;
