// Package config loads runtime configuration from the environment (12-factor).
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	Port            string
	DevUserID       string
	AuthValidateURL string
	AuthLoginURL    string
	AuthLogoutURL   string
	CookieDomain    string
	ClientDir       string
}

func Load() Config {
	// Best-effort: load a .env from the working dir for local dev. Real env vars
	// (shell, Docker) take precedence — godotenv never overrides what's set.
	_ = godotenv.Load()

	devUser := os.Getenv("DEV_USER_ID")
	validate := os.Getenv("AUTH_VALIDATE_URL")
	// Secure by default: only fall back to a dev user when NO central auth is
	// configured (pure local dev). With AUTH_VALIDATE_URL set, an unauthenticated
	// request gets 401 (→ the SPA redirects to AUTH_LOGIN_URL) unless DEV_USER_ID
	// is explicitly provided.
	if devUser == "" && validate == "" {
		devUser = "00000000-0000-0000-0000-000000000001"
	}

	return Config{
		DatabaseURL:     env("DATABASE_URL", "postgres://calories_user:password@localhost:5432/calories?sslmode=disable"),
		Port:            env("PORT", "8080"),
		DevUserID:       devUser,
		AuthValidateURL: validate,
		AuthLoginURL:    os.Getenv("AUTH_LOGIN_URL"),
		AuthLogoutURL:   os.Getenv("AUTH_LOGOUT_URL"),
		// Parent domain the auth cookies live on (e.g. .meizuno.com) so logout can
		// clear them; empty = host-only (local dev).
		CookieDomain: os.Getenv("COOKIE_DOMAIN"),
		// Directory of the built client (Vite dist) to serve. Empty in API-only
		// dev; set by the Docker image. The server never embeds the client.
		ClientDir: os.Getenv("CLIENT_DIR"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
