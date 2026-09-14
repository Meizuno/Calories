-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, family, user_agent, expires_at)
VALUES ($1, $2, COALESCE(sqlc.narg('family')::text, gen_random_uuid()::text), $3, $4)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: UseRefreshToken :execrows
UPDATE refresh_tokens SET used_at = now()
WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE family = $1 AND revoked_at IS NULL;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens WHERE expires_at < now() - interval '30 days';
