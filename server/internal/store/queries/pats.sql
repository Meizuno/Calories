-- name: CreatePat :one
INSERT INTO personal_access_tokens (profile_id, name, token_hash, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPatByHash :one
SELECT * FROM personal_access_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListPats :many
SELECT * FROM personal_access_tokens
WHERE profile_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokePat :execrows
UPDATE personal_access_tokens SET revoked_at = now()
WHERE id = $1 AND profile_id = $2 AND revoked_at IS NULL;

-- name: TouchPat :exec
UPDATE personal_access_tokens SET last_used_at = now() WHERE id = $1;
