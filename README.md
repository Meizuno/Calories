# Calories — kalorie tracker

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
  -e DEV_USER_ID=00000000-0000-0000-0000-000000000001 \
  calories                       # → http://localhost:8080
```

In production CI (`.github/workflows/deploy.yml`) builds & publishes the image to
`ghcr.io/meizuno/calories`, and the **Infrastructure** repo deploys it (Traefik
host `calories.meizuno.com`, per-app `calories_user` Postgres role, single sign-on
via the auth service). Migrations run on boot.

## Dev (separate processes)

```bash
# server (API on :8080; "/" shows an API-only notice without CLIENT_DIR)
cd server && go run ./cmd/server

# client (Vite on :5173, proxies /api → :8080)
cd client && pnpm install && pnpm dev

# optional: seed the dev user with sample foods + today's meals (local only)
cd server && go run ./cmd/seed       # or: make seed
```

`cmd/seed` is a dev-only tool — the Docker image builds only `./cmd/server`, so
it never ships in production.

## Client

Vue 3 SPA (Nuxt UI). Routes: `/` welcome (public), `/diary` the day (kcal ring +
per-macro bars, free-text add/edit-item form, meal accordion with inline rename
& row editing), `/catalog` the food catalog, `/profiles/me` own profile (also the
first-run onboarding form), and `/profile/:uuid` a public read-only view of a
*shared* profile's diary. Charts are dependency-free SVG (`components/RingChart.vue`,
`MacroBars.vue`).

The SPA bootstraps from `GET /api/session` (authenticated? profile? login URL).
Protected API calls 401 when there is no session; the client then redirects to
`AUTH_LOGIN_URL`. A freshly-created profile (`onboarded = false`) is sent to the
onboarding form before it can use the app.

## Stack notes
- **server:** `cmd/server` (migrate + serve), `internal/domain` (pure rules),
  `internal/store` (pgx + sqlc, embedded migrations), `internal/service`
  (Catalog, Diary), `internal/web` (chi, `/validate` auth, JSON `api.go`,
  `spa.go` serving the client dir).
- **client:** Vue 3 + Vite + **Nuxt UI** (Vue mode: `@nuxt/ui/vite` plugin +
  `@nuxt/ui/vue-plugin`), package manager **pnpm**.

## Profiles & auth
Everything hangs off a **profile** (`profiles` table): it links the external SSO
user (`user_id`), owns the daily goal, a display `name`, a `shared` flag and an
opaque `public_id` (the `/profile/{uuid}` sharing handle). A request resolves
`user_id → profile_id` once in middleware (`EnsureProfile`); every handler then
works on `profile_id`.

Same-origin, so the shared `access_token` cookie reaches the API automatically;
`auth.go` validates it against the auth service (`/validate`, set
`AUTH_VALIDATE_URL`) and 401s when absent — the SPA redirects to `AUTH_LOGIN_URL`
(and `AUTH_LOGOUT_URL` for sign-out). Locally, with no `AUTH_VALIDATE_URL`, it
falls back to `DEV_USER_ID`. Deploy the single image as a `calories` service
behind Traefik (`calories.<domain>`) with a `calories_user` Postgres role.

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
