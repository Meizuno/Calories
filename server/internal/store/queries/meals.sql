-- name: ListMealsForDay :many
SELECT * FROM meals WHERE profile_id = $1 AND date = $2 ORDER BY position, id;

-- name: MaxMealPosition :one
SELECT COALESCE(MAX(position), -1)::int AS pos FROM meals WHERE profile_id = $1 AND date = $2;

-- name: GetMealForProfile :one
SELECT * FROM meals WHERE id = $1 AND profile_id = $2;

-- name: CreateMeal :one
INSERT INTO meals (profile_id, date, name, position) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateMeal :exec
UPDATE meals SET name = $3 WHERE id = $1 AND profile_id = $2;

-- name: DeleteMeal :exec
DELETE FROM meals WHERE id = $1 AND profile_id = $2;
