-- name: ListEntriesForDay :many
SELECT e.* FROM entries e
JOIN meals m ON m.id = e.meal_id
WHERE m.profile_id = $1 AND m.date = $2
ORDER BY e.meal_id, e.position, e.id;

-- name: MaxEntryPosition :one
SELECT COALESCE(MAX(position), -1)::int AS pos FROM entries WHERE meal_id = $1;

-- name: DailyTotals :many
-- Per-day macro sums for a profile within an inclusive date range. Only days
-- that have at least one entry are returned; the client fills the gaps so the
-- chart has a continuous day-by-day axis.
SELECT
    m.date::date                        AS date,
    COALESCE(SUM(e.kcal), 0)::float8    AS kcal,
    COALESCE(SUM(e.carb), 0)::float8    AS carb,
    COALESCE(SUM(e.protein), 0)::float8 AS protein,
    COALESCE(SUM(e.fat), 0)::float8     AS fat
FROM meals m
JOIN entries e ON e.meal_id = m.id
WHERE m.profile_id = sqlc.arg(profile_id)
  AND m.date >= sqlc.arg(from_date)
  AND m.date <= sqlc.arg(to_date)
GROUP BY m.date
ORDER BY m.date;

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
