package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handlers, auth *Auth, profile *ProfileResolver, clientDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)

	// JSON API — same origin as the SPA, so the access_token cookie is sent
	// automatically.
	r.Route("/api", func(r chi.Router) {
		// Public — no session required.
		//   /session         bootstraps the SPA (authenticated? profile? login URL)
		//   /shared/{uuid}   read-only view of a profile that opted into sharing
		r.Get("/session", h.Session)
		r.Get("/login", h.Login)
		r.Post("/logout", h.Logout)
		r.Get("/shared/{uuid}", h.SharedProfile)
		r.Get("/shared/{uuid}/day", h.SharedDay)

		// Protected — auth resolves the external user (401 → SPA redirects to login)
		// and the profile resolver maps it to the local profile id every handler uses.
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware, profile.Middleware)

			r.Get("/profile", h.GetMyProfile)
			r.Put("/profile", h.SaveProfile)

			r.Get("/day", h.GetDay)
			r.Post("/meals", h.AddMeal)
			r.Patch("/meals/{id}", h.UpdateMeal)
			r.Delete("/meals/{id}", h.DeleteMeal)
			r.Post("/entries", h.AddEntry)
			r.Patch("/entries/{id}", h.UpdateEntry)
			r.Delete("/entries/{id}", h.DeleteEntry)

			r.Get("/foods", h.ListFoods)
			r.Post("/foods", h.CreateFood)
			r.Delete("/foods/{id}", h.DeleteFood)
		})
	})

	// Everything else → the built client (served from CLIENT_DIR at runtime).
	r.Handle("/*", SPAHandler(clientDir))
	return r
}
