package main

import (
	"context"
	"errors"
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

	auth := web.NewAuth(cfg.AuthValidateURL, cfg.DevUserID)
	h := web.NewHandlers(diary, catalog, profiles, auth, cfg.AuthLoginURL, cfg.AuthLogoutURL, cfg.CookieDomain)
	resolver := web.NewProfileResolver(profiles)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.NewRouter(h, auth, resolver, cfg.ClientDir),
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
