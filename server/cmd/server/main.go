package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meizuno/calories/config"
	"github.com/Meizuno/calories/internal/assistant"
	"github.com/Meizuno/calories/internal/ratelimit"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
	"github.com/Meizuno/calories/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// `calories -healthcheck` probes the running server's /health and exits 0/1.
	// The distroless image has no shell or curl, so the Docker HEALTHCHECK invokes
	// the binary itself. Handled before config.Load so the probe needs no DB/env
	// beyond PORT.
	healthcheck := flag.Bool("healthcheck", false, "probe the local server's /health and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	cfg := config.Load()

	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		fatal("migrations", err)
	}
	slog.Info("migrations applied")

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database", err)
	}
	defer st.Close()
	slog.Info("database connected")

	diary := service.NewDiary(st.Queries)
	catalog := service.NewCatalog(st.Queries)
	profiles := service.NewProfiles(st.Queries)
	tokens := service.NewTokens(st.Queries)
	authsvc := service.NewAuth(st.Queries, cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)

	google := web.NewGoogle(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL, cfg.SecureCookies)
	if google == nil {
		slog.Info("google sign-in disabled (GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL not set)")
	} else {
		slog.Info("google sign-in enabled", "allowed", cfg.GoogleAllowedEmails)
	}
	if !cfg.AllowRegistration {
		slog.Info("registration closed (set ALLOW_REGISTRATION=true to open it)")
	}
	auth := web.NewAuth(authsvc, cfg.SecureCookies)

	// The assistant is off unless a deployment opts in, because every message it
	// answers costs money. "mock" is the exception: it answers without calling
	// anyone, so the chat can be built and demonstrated for nothing.
	chat := buildAssistant(cfg, diary, catalog)
	h := web.NewHandlers(diary, catalog, profiles, tokens, auth, authsvc, google, cfg.GoogleAllowedEmails, cfg.AllowRegistration, chat)
	gate := web.NewGate(auth, profiles, tokens)

	// Rate limits, built once and shared: the auth routes are mounted at both
	// /api/v1/auth and /api/auth, and a limiter per mount would silently double
	// everyone's allowance. Refresh is deliberately far looser than sign-in —
	// rotating a session is ordinary traffic, and several tabs waking together
	// must not lock someone out of their own app.
	limits := web.Limits{
		Auth:    ratelimit.New(cfg.AuthRateLimit, cfg.AuthRateWindow),
		Refresh: ratelimit.New(cfg.AuthRateLimit*6, cfg.AuthRateWindow),
		// Keyed by profile rather than address: the bill follows the account.
		Assistant:  ratelimit.New(cfg.AssistantRateLimit, cfg.AssistantRateWindow),
		TrustProxy: cfg.TrustProxy,
	}
	slog.Info("auth rate limit",
		"attempts", cfg.AuthRateLimit, "per", cfg.AuthRateWindow, "trustProxy", cfg.TrustProxy)

	// Spent refresh rows accumulate as sessions rotate; drop the long-expired ones
	// daily so the table stays small.
	go purgeExpiredTokens(ctx, authsvc)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.NewRouter(h, gate, cfg.ClientDir, limits),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("serve", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
}

// purgeExpiredTokens sweeps spent refresh rows once at boot and daily after.
func purgeExpiredTokens(ctx context.Context, authsvc *service.Auth) {
	for {
		if err := authsvc.PurgeExpired(ctx); err != nil {
			slog.Warn("purge expired refresh tokens", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}

// runHealthcheck does a short-timeout GET to the local /health and returns a
// process exit code (0 = healthy). Used by the Docker HEALTHCHECK.
func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		slog.Error("healthcheck", "error", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	slog.Error("healthcheck", "status", resp.StatusCode)
	return 1
}

// buildAssistant picks the model behind the in-app chat, or none at all.
//
// A misconfiguration is fatal rather than silent: a deployment that meant to
// enable the assistant and instead ran without one would look fine until the
// first person asked it a question.
func buildAssistant(cfg config.Config, diary *service.Diary, catalog *service.Catalog) *assistant.Service {
	tools := assistant.Tools(diary, catalog)
	switch cfg.Assistant {
	case "":
		slog.Info("assistant disabled (set ASSISTANT=mock to try it without a key)")
		return nil
	case "mock":
		slog.Warn("assistant running on the MOCK provider - replies are canned, no model is called")
		return assistant.New(&assistant.MockProvider{Delay: 25 * time.Millisecond}, tools...)
	case "openai":
		if cfg.AssistantKey == "" {
			fatal("assistant", errors.New("ASSISTANT=openai needs ASSISTANT_API_KEY"))
		}
		p := assistant.NewOpenAI(cfg.AssistantKey, cfg.AssistantModel)
		// The model is logged because it is the one setting that silently
		// changes both the bill and the quality of every answer.
		slog.Info("assistant enabled", "provider", p.Name(), "model", p.Model)
		return assistant.New(p, tools...)
	default:
		fatal("assistant", fmt.Errorf("unknown ASSISTANT %q (known: mock, openai)", cfg.Assistant))
		return nil
	}
}
