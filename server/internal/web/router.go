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

	// JSON API — same origin as the SPA, so the access_token cookie is sent
	// automatically.
	r.Route("/api", func(r chi.Router) {
		// Public — no session required.
		//   /session         bootstraps the SPA (authenticated? profile?)
		//   /shared/{uuid}   read-only view of a profile that opted into sharing
		r.Get("/session", h.Session)
		r.Get("/login", h.Login)
		r.Post("/logout", h.Logout)
		r.Get("/shared/{uuid}", h.SharedProfile)
		r.Get("/shared/{uuid}/day", h.SharedDay)
		r.Get("/shared/{uuid}/stats", h.SharedStats)
		r.Get("/shared/{uuid}/days", h.SharedDays)

		// Protected. The Gate resolves either a full session (cookie, refreshed if
		// stale) or a scoped PAT, then Scope() enforces per-route access:
		//   "read" / "add" — a PAT may hold these;
		//   ""             — full-session only (PATs can't update/delete/manage).
		r.Group(func(r chi.Router) {
			r.Use(gate.Middleware)

			// Account & token management — full session only.
			r.With(gate.Scope("")).Get("/profile", h.GetMyProfile)
			r.With(gate.Scope("")).Put("/profile", h.SaveProfile)
			r.With(gate.Scope("")).Get("/pats", h.ListPats)
			r.With(gate.Scope("")).Post("/pats", h.CreatePat)
			r.With(gate.Scope("")).Delete("/pats/{id}", h.RevokePat)

			// Diary.
			r.With(gate.Scope("read")).Get("/day", h.GetDay)
			r.With(gate.Scope("read")).Get("/days", h.ListDays)
			r.With(gate.Scope("read")).Get("/stats", h.GetStats)
			r.With(gate.Scope("add")).Post("/meals", h.AddMeal)
			r.With(gate.Scope("")).Patch("/meals/{id}", h.UpdateMeal)
			r.With(gate.Scope("")).Delete("/meals/{id}", h.DeleteMeal)
			r.With(gate.Scope("add")).Post("/entries", h.AddEntry)
			r.With(gate.Scope("")).Patch("/entries/{id}", h.UpdateEntry)
			r.With(gate.Scope("")).Delete("/entries/{id}", h.DeleteEntry)

			// Machine-only: log a full meal (name + note + entries) in one call.
			// PAT only (a browser session is rejected) and needs the "add" scope —
			// the endpoint a chat assistant posts to.
			r.With(gate.PAT("add")).Post("/log", h.LogMeal)

			// Food catalog (no UI yet; kept for the future food picker).
			r.With(gate.Scope("read")).Get("/foods", h.ListFoods)
			r.With(gate.Scope("add")).Post("/foods", h.CreateFood)
			r.With(gate.Scope("")).Delete("/foods/{id}", h.DeleteFood)
		})
	})

	// Everything else → the built client (served from CLIENT_DIR at runtime).
	r.Handle("/*", SPAHandler(clientDir))
	return r
}
