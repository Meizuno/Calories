package web

import (
	"context"
	"net/http"

	"github.com/Meizuno/calories/internal/service"
)

// ProfileResolver maps the authenticated external user to a local profile
// (creating it on first request) and puts the profile id in the request context.
// It runs after Auth, so UserID is already populated.
type ProfileResolver struct {
	profiles *service.Profiles
}

func NewProfileResolver(profiles *service.Profiles) *ProfileResolver {
	return &ProfileResolver{profiles: profiles}
}

func (p *ProfileResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prof, err := p.profiles.Ensure(r.Context(), UserID(r.Context()))
		if err != nil {
			http.Error(w, "profile unavailable", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), profileIDKey, prof.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
