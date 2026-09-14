-- name: CreateUser :one
INSERT INTO users (email, email_norm, password_hash, name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email_norm = $1;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: GetIdentity :one
SELECT * FROM identities WHERE provider = $1 AND provider_user_id = $2;

-- name: LinkIdentity :one
INSERT INTO identities (user_id, provider, provider_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (provider, provider_user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: ListIdentities :many
SELECT * FROM identities WHERE user_id = $1 ORDER BY created_at;
