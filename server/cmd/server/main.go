package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meizuno/calories/config"
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

	auth := web.NewAuth(cfg.AuthValidateURL, cfg.AuthRefreshURL, cfg.DevUserID)
	h := web.NewHandlers(diary, catalog, profiles, tokens, auth, cfg.AuthLoginURL, cfg.AuthLogoutURL, cfg.CookieDomain)
	gate := web.NewGate(auth, profiles, tokens)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.NewRouter(h, gate, cfg.ClientDir),
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
