-- name: ListEntriesForDay :many
SELECT e.* FROM entries e
JOIN meals m ON m.id = e.meal_id
WHERE m.profile_id = $1 AND m.date = $2
ORDER BY e.meal_id, e.position, e.id;

-- name: MaxEntryPosition :one
SELECT COALESCE(MAX(position), -1)::int AS pos FROM entries WHERE meal_id = $1;

-- name: CreateEntry :one
INSERT INTO entries (meal_id, food_id, name, quantity, unit, position, kcal, carb, protein, fat)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateEntry :exec
UPDATE entries AS e
SET name = $3, quantity = $4, unit = $5, kcal = $6, carb = $7, protein = $8, fat = $9
FROM meals AS m
WHERE e.id = $1 AND e.meal_id = m.id AND m.profile_id = $2;

-- name: DeleteEntry :exec
DELETE FROM entries AS e USING meals AS m
WHERE e.id = $1 AND e.meal_id = m.id AND m.profile_id = $2;
