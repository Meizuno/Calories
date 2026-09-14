-- Auth moves in-house. Until now a profile pointed at an external SSO user id and
-- every request was validated against the auth service; now Calories owns the
-- accounts: `users` (email + optional password), `identities` (linked OAuth
-- logins) and `refresh_tokens` (opaque, rotating session handles).
--
-- Ids are text-uuid rather than the uuid type, matching profiles.public_id — it
-- keeps the generated sqlc layer on plain Go strings.

CREATE TABLE users (
    id            text PRIMARY KEY DEFAULT gen_random_uuid()::text,
    -- email is kept as typed for display; email_norm (lowercased) is the lookup
    -- key and carries the uniqueness constraint, so no citext extension is needed.
    email         text NOT NULL,
    email_norm    text NOT NULL UNIQUE,
    -- NULL for an account that only ever signed in with Google. Such a user can
    -- set a password later; a password user can likewise link Google.
    password_hash text,
    name          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- One row per external login linked to a user. provider_user_id is the provider's
-- stable subject (Google's `sub`) — never the email, which can change hands.
CREATE TABLE identities (
    id               bigserial PRIMARY KEY,
    user_id          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         text NOT NULL,
    provider_user_id text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX identities_user_idx ON identities (user_id);

-- Refresh tokens are opaque random strings; only the sha256 hash is stored (as
-- with personal_access_tokens). Each refresh rotates the token, and `family` ties
-- a rotation chain together: presenting an already-rotated token means it leaked,
-- so the whole family is revoked at once.
CREATE TABLE refresh_tokens (
    id         bigserial PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    family     text NOT NULL DEFAULT gen_random_uuid()::text,
    user_agent text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    used_at    timestamptz,
    revoked_at timestamptz
);
CREATE INDEX refresh_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_family_idx ON refresh_tokens (family);

-- profiles.user_id switches from "external SSO id" to "local users.id". Existing
-- rows carry ids this app can no longer resolve (the auth service knew the email,
-- we never did), so they are parked in legacy_user_id and the profile is left
-- unclaimed (user_id NULL) until `cmd/claim` attaches it to a real account.
ALTER TABLE profiles ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE profiles ADD COLUMN legacy_user_id text;
UPDATE profiles SET legacy_user_id = user_id, user_id = NULL;
ALTER TABLE profiles ADD CONSTRAINT profiles_user_fk
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;
CREATE UNIQUE INDEX profiles_legacy_user_idx
    ON profiles (legacy_user_id) WHERE legacy_user_id IS NOT NULL;
