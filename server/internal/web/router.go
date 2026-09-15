package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handlers, gate *Gate, clientDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	// Liveness/readiness probe — unauthenticated, no DB touch. Backs the Docker
	// HEALTHCHECK (the binary self-probes this via `-healthcheck`) so `docker
	// rollout` waits for a healthy new container before removing the old one.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// The API lives under a version. Everything in it answers with a JSON
		// object naming what it returns ({"day":…}, {"foods":…}) or, on failure,
		// {"code","message"} with a real status — so a response can gain a field
		// without breaking a caller, and a failed write never looks like a
		// successful one.
		r.Route("/v1", func(r chi.Router) {
			mountAuth(r, h, gate)
			mountShared(r, h)
			mountProtected(r, h, gate)
		})

		// Unversioned aliases. These two are configured OUTSIDE this repository
		// and cannot be moved by deploying:
		//   /api/auth/*  the redirect URL registered on the Google OAuth client
		//   /api/log     pasted into whatever assistant posts meals
		// Everything else moved to /v1; the SPA ships with the server, so it was
		// updated in step.
		mountAuth(r, h, gate)
		r.With(gate.Middleware, gate.PAT("add")).Post("/log", h.CreateMeal)
	})

	// Everything else → the built client (served from CLIENT_DIR at runtime).
	r.Handle("/*", SPAHandler(clientDir))
	return r
}

// mountAuth registers the session endpoints. Called twice: once under /v1 and
// once unversioned, because Google holds the callback URL and a deploy cannot
// change what is registered there.
func mountAuth(r chi.Router, h *Handlers, gate *Gate) {
	// Bootstraps a client: authenticated? which account? which profile? which
	// sign-in methods does this deployment offer?
	r.Get("/session", h.Session)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.LoginPassword)
		// Rotates the session. The refresh cookie is scoped to /api, which covers
		// this path under both the versioned and unversioned mount.
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		// Google sign-in is two top-level navigations: out to the consent screen,
		// back to the callback. Never fetched.
		r.Get("/google", h.GoogleStart)
		r.Get("/google/callback", h.GoogleCallback)
		// Changing a password needs a live session, not a PAT.
		r.With(gate.Middleware, gate.Scope("")).Post("/password", h.ChangePassword)
	})
}

// mountShared is the public, read-only view of a profile that opted into
// sharing. SharedProfileCtx resolves the uuid and puts the profile id where the
// Gate would have put it, so these are the SAME handlers the owner uses rather
// than a parallel set that drifts.
func mountShared(r chi.Router, h *Handlers) {
	r.Route("/shared/{uuid}", func(r chi.Router) {
		r.Use(h.SharedProfileCtx)
		r.Get("/", h.SharedProfile)
		r.Get("/day", h.GetDay)
		r.Get("/days", h.ListDays)
		r.Get("/stats", h.GetStats)
	})
}

// mountProtected needs a principal: either a full session (cookie, refreshed if
// stale) or a scoped PAT. Scope() then enforces what that principal may do —
// default-deny, so an operation marked "" is full-session only and a PAT can
// never reach it.
func mountProtected(r chi.Router, h *Handlers, gate *Gate) {
	r.Group(func(r chi.Router) {
		r.Use(gate.Middleware)

		// Account & token management — full session only.
		r.With(gate.Scope("")).Get("/profile", h.GetMyProfile)
		r.With(gate.Scope("")).Put("/profile", h.SaveProfile)
		r.With(gate.Scope("")).Get("/pats", h.ListPats)
		r.With(gate.Scope("")).Post("/pats", h.CreatePat)
		r.With(gate.Scope("")).Delete("/pats/{id}", h.RevokePat)

		// Reading the diary.
		r.With(gate.Scope("read")).Get("/day", h.GetDay)
		r.With(gate.Scope("read")).Get("/days", h.ListDays)
		r.With(gate.Scope("read")).Get("/stats", h.GetStats)

		// Meals. Creating one optionally carries its entries, so an assistant
		// logs a whole meal in one call and the diary posts a bare one — the
		// same endpoint either way.
		r.Route("/meals", func(r chi.Router) {
			r.With(gate.Scope("add")).Post("/", h.CreateMeal)
			r.With(gate.Scope("")).Patch("/{id}", h.UpdateMeal)
			r.With(gate.Scope("")).Delete("/{id}", h.DeleteMeal)
			// Repeat a meal on another day. "add": it only ever creates.
			r.With(gate.Scope("add")).Post("/{id}/copy", h.CopyMeal)
			// An entry is created against its meal, which is an address rather
			// than a body field.
			r.With(gate.Scope("add")).Post("/{id}/entries", h.AddEntry)
		})

		// An entry already has a unique id, so editing and removing one needs no
		// parent in the path; ownership is enforced through the meal in SQL.
		r.With(gate.Scope("")).Patch("/entries/{id}", h.UpdateEntry)
		r.With(gate.Scope("")).Delete("/entries/{id}", h.DeleteEntry)

		// Remembered foods, built from what has been logged.
		r.With(gate.Scope("read")).Get("/foods", h.ListFoods)
		r.With(gate.Scope("add")).Post("/foods", h.CreateFood)
		r.With(gate.Scope("")).Delete("/foods/{id}", h.DeleteFood)
	})
}
