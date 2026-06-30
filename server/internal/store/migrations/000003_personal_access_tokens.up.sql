-- Personal access tokens for programmatic API access (Authorization: Bearer
-- cal_pat_…). Only the sha256 hash is stored; the raw token is shown once at
-- creation. A PAT is limited to its `scopes`; a real session is full access.
CREATE TABLE personal_access_tokens (
    id           bigserial PRIMARY KEY,
    profile_id   bigint NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    name         text NOT NULL,
    token_hash   text NOT NULL UNIQUE,
    scopes       text[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at   timestamptz,
    revoked_at   timestamptz
);
CREATE INDEX pat_profile_idx ON personal_access_tokens (profile_id);
