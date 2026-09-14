# Calories — kalorie tracker

<p align="center">
  <img src="preview/demo.gif" alt="Calories demo — macro diary and the weekly/monthly statistics" width="760"><br>
  <sub>▶ <a href="preview/demo.mp4">full-resolution MP4</a></sub>
</p>

Calorie/macro diary. **Clear client/server split** at the repo root; they are
independent projects, combined only at build time.

```
calories/
  server/    Go JSON API (chi, pgx + sqlc, golang-migrate)   — module, builds standalone
  client/    Vue 3 + Vite + Nuxt UI (pnpm)                    — SPA, builds standalone
  Dockerfile                                                  — combines the two into one image (deployed via Infrastructure)
```

The server never embeds the client. At build the Docker image builds each, then
places the client's `dist/` beside the binary; the server serves it from
`CLIENT_DIR` (single-origin → no CORS). In dev they run as two processes.

## Build & deploy

A single Docker image bundles both (multi-stage `Dockerfile`: build client → build
server → distroless). Build/run locally:

```bash
docker build -t calories .
docker run --rm -p 8080:8080 \
  -e DATABASE_URL=postgres://calories_user:password@host.docker.internal:5432/calories?sslmode=disable \
  -e JWT_SECRET="$(openssl rand -base64 48)" \
  calories                       # → http://localhost:8080
```

In production CI (`.github/workflows/deploy.yml`) builds & publishes the image to
`ghcr.io/meizuno/calories`, and the **Infrastructure** repo deploys it (Traefik
host `calories.meizuno.com`, per-app `calories_user` Postgres role). Migrations
run on boot — set `JWT_SECRET` and the `GOOGLE_*` vars on the service.

## Dev

Both at once, with live reload on each — **this is the normal way to run it**:

```powershell
.\dev.ps1          # server + client, merged output, Ctrl+C stops both
.\dev.ps1 -SkipClient   # API only
```

Open **http://localhost:5173** (not :8080) — the Vite proxy puts `/api` on the
same origin, which the session cookies depend on. The Go side rebuilds and
restarts on every `.go`/`.sql`/`.env` save via [air](https://github.com/air-verse/air)
(`server/.air.toml`); the SPA hot-reloads in the browser. One-time setup:

```bash
go install github.com/air-verse/air@latest
cd client && pnpm install
```

Or run the two processes by hand:

```bash
# server (API on :8080; "/" shows an API-only notice without CLIENT_DIR)
cd server && go run ./cmd/server     # or: air   — to get live reload

# client (Vite on :5173, proxies /api → :8080)
cd client && pnpm install && pnpm dev

# optional: seed a dev ACCOUNT with sample foods + today's meals (local only)
cd server && go run ./cmd/seed       # or: make seed
```

`cmd/seed` creates `dev@example.com` / `devpassword` (override with
`SEED_EMAIL`/`SEED_PASSWORD`) and fills its diary — sign in with those. It is a
dev-only tool: the Docker image builds only `./cmd/server`, so neither it nor
`cmd/claim` ships in production.

## Client

Vue 3 SPA (Nuxt UI). Routes: `/` welcome (public), `/login` sign-in & sign-up
(public), `/diary` the day (kcal ring +
per-macro bars, free-text add/edit-item form, meal accordion with inline rename
& row editing), `/catalog` the food catalog, `/profiles/me` own profile (also the
first-run onboarding form), and `/profile/:uuid` a public read-only view of a
*shared* profile's diary. Charts are dependency-free SVG (`components/RingChart.vue`,
`MacroBars.vue`).

The SPA bootstraps from `GET /api/session` (authenticated? which account? which
profile? is Google sign-in available?). Every API call goes through `lib/http.ts`:
on a 401 it calls `POST /api/auth/refresh` **once**, replays the request, and only
then sends the visitor to `/login` — concurrent 401s share a single refresh, so
parallel requests can't rotate the token against each other. A freshly-created
profile (`onboarded = false`) is sent to the onboarding form before it can use
the app.

## Stack notes
- **server:** `cmd/server` (migrate + serve), `internal/domain` (pure rules),
  `internal/store` (pgx + sqlc, embedded migrations), `internal/service`
  (Catalog, Diary, Auth), `internal/web` (chi, `auth.go` JWT middleware,
  `authapi.go` sign-in endpoints, `oauth.go` Google, JSON `api.go`, `spa.go`
  serving the client dir).
- **client:** Vue 3 + Vite + **Nuxt UI** (Vue mode: `@nuxt/ui/vite` plugin +
  `@nuxt/ui/vue-plugin`), package manager **pnpm**.

## Accounts & auth
Auth is **in-app** — there is no external auth service. Three tables:

- **`users`** — the account: `email` (matched on a lowercased `email_norm`),
  an optional `password_hash` (NULL = Google-only), `name`.
- **`identities`** — one row per linked OAuth login, keyed by the provider's
  stable subject (Google's `sub`), never the email.
- **`refresh_tokens`** — opaque session handles; only the sha256 hash is stored.

A **profile** (`profiles.user_id → users.id`) owns the daily goal, a display
`name`, a `shared` flag and an opaque `public_id` (the `/profile/{uuid}` sharing
handle). Middleware resolves `user_id → profile_id` once (`EnsureProfile`); every
handler then works on `profile_id`.

### Sessions
Two HttpOnly cookies, so no token is ever reachable from page script:

| cookie | what | path | lifetime |
|---|---|---|---|
| `access_token` | signed JWT (HS256, `sub` = user id) | `/` | `ACCESS_TTL`, default 15m |
| `refresh_token` | opaque random, sha256-hashed in the DB | `/api/auth` | `REFRESH_TTL`, default 30d |

Verifying a request is a signature check — no database round-trip. When the access
token expires the API 401s, the SPA calls `POST /api/auth/refresh`, and the request
is replayed.

Refresh tokens **rotate**: each refresh issues a new one and marks the old used, in
a single statement that also asserts it was unused — so two concurrent refreshes
cannot both win. Tokens carry a `family` id; presenting an already-rotated token
means it leaked, and the whole family is revoked at once, signing out attacker and
victim together. `/api/auth/logout` revokes the family and clears both cookies.

Because the access token is stateless, a revoked session keeps working until that
token expires — which is why `ACCESS_TTL` is short. Endpoints:

```
POST /api/auth/register   {email, password, name}   → session, sets cookies
POST /api/auth/login      {email, password}         → session, sets cookies
POST /api/auth/refresh                              → rotates, sets cookies
POST /api/auth/logout                               → revokes family, clears cookies
GET  /api/auth/google                               → redirect to Google (navigation)
GET  /api/auth/google/callback                      → completes sign-in
POST /api/auth/password   {current, new}            → change/set, revokes other sessions
```

### Who may sign in
The app is single-user, so both doors are shut by default:

- **Sign-up is closed.** `POST /api/auth/register` returns 403 unless
  `ALLOW_REGISTRATION=true`. The SPA hides the sign-up form and the "create an
  account" links when it is closed, so nobody meets that 403. The gate sits in
  the HTTP handler, not the service — `cmd/seed` creates the dev account through
  `service.Auth` directly and keeps working.
- **Google is allowlisted.** Only the addresses in `GOOGLE_ALLOWED_EMAILS`
  (default: the single address in `config.go`) can complete the flow; anyone else
  is bounced to `/login` with an error, and **no account is created** — the check
  runs before the user is touched.

Together these close the takeover path that open sign-up would otherwise create:
with no email verification, anyone could register an address before its real
owner first signed in with Google, and the Google identity would then link onto
that pre-existing account.

### Google sign-in
Authorization Code flow; the app is a confidential client, and CSRF is covered by
a `state` value echoed in a short-lived cookie. Accounts link by **verified** email:
signing in with Google on an address that already has a password account attaches
the identity rather than creating a duplicate. Unset `GOOGLE_CLIENT_ID` to disable
it — `/api/session` reports `google: false` and the SPA drops the button.

### Configuration
| var | meaning |
|---|---|
| `JWT_SECRET` | signs the access token; **required in prod**, ≥ 32 chars. Unset → a random per-boot secret, so every restart signs everyone out |
| `ACCESS_TTL` / `REFRESH_TTL` | Go durations; default `15m` / `720h` |
| `SECURE_COOKIES` | mark cookies `Secure`; defaults on when `CLIENT_DIR` is set (the Docker image), off for plain-HTTP local dev |
| `GOOGLE_CLIENT_ID` / `_SECRET` / `_REDIRECT_URL` | Google OAuth client; the redirect URL must match the one registered on it exactly |
| `GOOGLE_ALLOWED_EMAILS` | comma-separated sign-in allowlist; defaults to the single address in `config.go` |
| `ALLOW_REGISTRATION` | open `POST /api/auth/register`; default `false` |

Deploy the single image as a `calories` service behind Traefik
(`calories.<domain>`) with a `calories_user` Postgres role.

### Migrating off the old SSO
Migration `000004` moves `profiles.user_id` from the external SSO id onto
`users.id`. Those old ids carry no email and the auth service is gone, so nothing
can resolve them automatically: each is preserved in `profiles.legacy_user_id` and
its profile left unclaimed. Register the account in the app, then hand the diary
over:

```bash
cd server
go run ./cmd/claim                                    # list unclaimed profiles
go run ./cmd/claim -profile 1 -email me@example.com   # hand profile 1 over
```

The empty profile the new account got at signup is deleted in the process, since a
user owns exactly one.

## Personal access tokens (server-only)
Programmatic API access without a browser. A PAT is sent as
`Authorization: Bearer cal_pat_…` and is scoped to `read` (read the diary) and/or
`add` (add meals/entries). Update, delete and all account/token management are
**full-session only** (default-deny — a PAT can never reach them). Only the
sha256 hash is stored (`personal_access_tokens`); the raw token is shown once.

There is no UI — manage tokens over the API with a logged-in session cookie
(`$C` = your `access_token`):

```bash
# create — the raw token is returned ONCE
curl -X POST https://calories.meizuno.com/api/pats -b "access_token=$C" \
  -H 'Content-Type: application/json' -d '{"name":"import","scopes":["read","add"]}'
curl https://calories.meizuno.com/api/pats -b "access_token=$C"          # list
curl -X DELETE https://calories.meizuno.com/api/pats/<id> -b "access_token=$C"  # revoke

# use the PAT (no cookie):
curl https://calories.meizuno.com/api/day -H "Authorization: Bearer cal_pat_…"
```

### Log a meal — `POST /api/log` (PAT only, scope `add`)
The endpoint a chat assistant posts to: it creates a whole meal — name, optional
note and its entries — in one call, and returns the updated day. **PAT only** — a
browser session is rejected (use the diary UI for that). Macros are per the whole
entry as stated (not per 100 g); the server clamps negatives to 0 and skips
entries without a name or with a non-positive quantity.

```bash
curl -X POST https://calories.meizuno.com/api/log \
  -H "Authorization: Bearer cal_pat_…" -H 'Content-Type: application/json' -d '{
    "date": "2026-06-30",
    "meal": "Oběd",
    "note": "doma",
    "entries": [
      {"name": "Kuřecí prsa", "quantity": 200, "unit": "g", "kcal": 330, "carb": 0,  "protein": 62, "fat": 7},
      {"name": "Rýže",        "quantity": 150, "unit": "g", "kcal": 195, "carb": 42, "protein": 4,  "fat": 1}
    ]
  }'
```

Request body — the JSON Schema to hand the assistant (e.g. as a tool definition):

```json
{
  "type": "object",
  "required": ["meal", "entries"],
  "additionalProperties": false,
  "properties": {
    "date": {
      "type": "string", "format": "date",
      "description": "YYYY-MM-DD; defaults to today (UTC) if omitted"
    },
    "meal": {
      "type": "string", "minLength": 1,
      "description": "Meal name, e.g. Snídaně / Oběd / Večeře / Svačina"
    },
    "note": { "type": "string", "description": "Optional free-text note for the meal" },
    "entries": {
      "type": "array", "minItems": 1,
      "items": {
        "type": "object",
        "required": ["name", "quantity"],
        "additionalProperties": false,
        "properties": {
          "name":     { "type": "string", "minLength": 1, "description": "Food name" },
          "quantity": { "type": "number", "exclusiveMinimum": 0, "description": "Amount eaten, in `unit`" },
          "unit":     { "type": "string", "default": "g", "description": "g, ml, ks, porce, …" },
          "kcal":     { "type": "number", "minimum": 0, "description": "Calories for this entry" },
          "carb":     { "type": "number", "minimum": 0, "description": "Carbohydrates (g)" },
          "protein":  { "type": "number", "minimum": 0, "description": "Protein (g)" },
          "fat":      { "type": "number", "minimum": 0, "description": "Fat (g)" }
        }
      }
    }
  }
}
```
