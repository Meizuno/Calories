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

-- name: DeleteFood :exec
DELETE FROM foods WHERE id = $1 AND profile_id = $2;
