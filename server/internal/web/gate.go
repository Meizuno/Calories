package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/Meizuno/calories/internal/service"
)

// Gate authenticates a protected request as either a full session (cookie access
// token, refreshed transparently) or a scoped PAT (Authorization: Bearer
// cal_pat_…), and stores the profile id + scopes in the context. Per-route
// Scope() then enforces what that principal may do — default-deny: an operation
// with no scope is full-session only, so a PAT can never reach it.
type Gate struct {
	auth     *Auth
	profiles *service.Profiles
	tokens   *service.Tokens
}

func NewGate(auth *Auth, profiles *service.Profiles, tokens *service.Tokens) *Gate {
	return &Gate{auth: auth, profiles: profiles, tokens: tokens}
}

func (g *Gate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PAT: Authorization: Bearer cal_pat_… → scoped principal.
		if authz := r.Header.Get("Authorization"); strings.HasPrefix(authz, "Bearer ") {
			tok := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
			if strings.HasPrefix(tok, service.PATPrefix) {
				pid, scopes, ok := g.tokens.Resolve(r.Context(), tok)
				if !ok {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), profileIDKey, pid)
				ctx = context.WithValue(ctx, scopesKey, scopes)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Otherwise a full session (cookie access token, refreshed if stale).
		uid := g.auth.ResolveWithRefresh(w, r)
		if uid == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		prof, err := g.profiles.Ensure(r.Context(), uid)
		if err != nil {
			http.Error(w, "profile unavailable", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		ctx = context.WithValue(ctx, profileIDKey, prof.ID)
		ctx = context.WithValue(ctx, fullKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Scope guards a route. A full session passes anything; a PAT must hold the named
// scope. An empty scope marks a full-session-only operation.
func (g *Gate) Scope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed(r.Context(), scope) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func allowed(ctx context.Context, scope string) bool {
	if IsFull(ctx) {
		return true
	}
	if scope == "" {
		return false
	}
	for _, s := range Scopes(ctx) {
		if s == scope {
			return true
		}
	}
	return false
}
