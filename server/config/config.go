// Package config loads runtime configuration from the environment (12-factor).
package config

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
	ClientDir   string

	// JWTSecret signs the access token (HS256). Rotating it invalidates every
	// issued access token; refresh tokens live in the database and survive.
	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	// SecureCookies marks the session cookies Secure. Off for plain-HTTP local
	// dev, since a browser drops Secure cookies on http://localhost.
	SecureCookies bool

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	// GoogleAllowedEmails is the sign-in allowlist: only these addresses may
	// complete the Google flow. Empty would mean "anyone", which this app never
	// wants, so Load always falls back to defaultAllowedEmail.
	GoogleAllowedEmails []string

	// AllowRegistration opens POST /api/auth/register. Off by default: this is a
	// single-user app, and open registration combined with no email verification
	// would let anyone claim an address before its real owner signs in with
	// Google. The seed tool creates the dev account through the service directly,
	// so it is unaffected by this.
	AllowRegistration bool

	// AuthRateLimit caps how many sign-in attempts one client IP may make in
	// AuthRateWindow. Every attempt costs a bcrypt hash, so this bounds guessing
	// and CPU burn alike. Configurable mostly so you can raise it if you ever
	// lock yourself out.
	AuthRateLimit  int
	AuthRateWindow time.Duration

	// TrustProxy says whether X-Forwarded-For may be believed when identifying a
	// caller. True behind our own reverse proxy, false when the server is exposed
	// directly — where anyone could otherwise hand themselves a fresh rate-limit
	// bucket per request. Defaults the same way SecureCookies does: on in the
	// Docker image (which always runs behind Caddy), off in local dev.
	TrustProxy bool

	// Assistant selects the model behind the in-app chat: "" (off), "mock" (a
	// canned reply, for building the UI without a key or a bill) or a provider
	// name. Off by default -- a deployment should opt in to spending money.
	Assistant string
	// AssistantKey is the provider's API key. Without one, a real provider is
	// refused at startup rather than failing on the first message.
	AssistantKey string
	// AssistantReasoning is sent as reasoning_effort. Empty omits it entirely,
	// which is what a model predating the parameter needs; the provider's own
	// default is "none", because tools and reasoning cannot be combined on the
	// endpoint this talks to.
	AssistantReasoning string
	// AssistantModel overrides the provider's default model.
	AssistantModel string
	// AssistantRateLimit caps messages per profile per AssistantRateWindow. This
	// is the cost ceiling: every message is a paid call, possibly several.
	AssistantRateLimit  int
	AssistantRateWindow time.Duration
}

// The one account allowed to sign in with Google. Override with
// GOOGLE_ALLOWED_EMAILS (comma-separated) rather than editing this.
const defaultAllowedEmail = "yuramiron16@gmail.com"

func Load() Config {
	// Best-effort: load a .env from the working dir for local dev. Real env vars
	// (shell, Docker) take precedence — godotenv never overrides what's set.
	_ = godotenv.Load()

	return Config{
		DatabaseURL: env("DATABASE_URL", "postgres://calories_user:password@localhost:5432/calories?sslmode=disable"),
		Port:        env("PORT", "8080"),
		// Directory of the built client (Vite dist) to serve. Empty in API-only
		// dev; set by the Docker image. The server never embeds the client.
		ClientDir: os.Getenv("CLIENT_DIR"),

		JWTSecret: jwtSecret(),
		// Short access token, long refresh: a revoked session stops working within
		// AccessTTL, while a normal user stays signed in for REFRESH_TTL of
		// inactivity (every refresh issues a new one).
		AccessTTL:     duration("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:    duration("REFRESH_TTL", 30*24*time.Hour),
		SecureCookies: boolEnv("SECURE_COOKIES", os.Getenv("CLIENT_DIR") != ""),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		// Must match a redirect URI registered on the Google OAuth client exactly,
		// e.g. https://calories.meizuno.com/api/auth/google/callback.
		GoogleRedirectURL:   os.Getenv("GOOGLE_REDIRECT_URL"),
		GoogleAllowedEmails: allowedEmails(),

		AllowRegistration: boolEnv("ALLOW_REGISTRATION", false),

		AuthRateLimit:  intEnv("AUTH_RATE_LIMIT", 10),
		AuthRateWindow: duration("AUTH_RATE_WINDOW", 15*time.Minute),
		TrustProxy:     boolEnv("TRUST_PROXY", os.Getenv("CLIENT_DIR") != ""),

		Assistant:           strings.ToLower(strings.TrimSpace(os.Getenv("ASSISTANT"))),
		AssistantKey:        os.Getenv("ASSISTANT_API_KEY"),
		AssistantModel:      os.Getenv("ASSISTANT_MODEL"),
		AssistantReasoning:  envSet("ASSISTANT_REASONING", "none"),
		AssistantRateLimit:  intEnv("ASSISTANT_RATE_LIMIT", 30),
		AssistantRateWindow: duration("ASSISTANT_RATE_WINDOW", time.Hour),
	}
}

// allowedEmails parses GOOGLE_ALLOWED_EMAILS (comma-separated, case-insensitive)
// and falls back to the single built-in address.
func allowedEmails() []string {
	raw := os.Getenv("GOOGLE_ALLOWED_EMAILS")
	if strings.TrimSpace(raw) == "" {
		return []string{defaultAllowedEmail}
	}
	var out []string
	for _, e := range strings.Split(raw, ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return []string{defaultAllowedEmail}
	}
	return out
}

// jwtSecret returns JWT_SECRET, or a random per-boot secret for local dev. A
// generated secret means every restart signs out everyone, which is fine for dev
// and loud enough to catch a missing production value.
func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); len(s) >= 32 {
		return s
	}
	if s := os.Getenv("JWT_SECRET"); s != "" {
		slog.Warn("JWT_SECRET is shorter than 32 characters — generating a temporary one instead")
	} else {
		slog.Warn("JWT_SECRET is not set — generating a temporary one; sessions will not survive a restart")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("cannot generate a jwt secret: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envSet is env() for a setting whose empty value is meaningful. Setting
// ASSISTANT_REASONING= must switch the parameter OFF, not fall back to the
// default, so unset and empty have to be told apart.
func envSet(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		slog.Warn("ignoring invalid value", "key", key, "value", v)
		return fallback
	}
	return n
}

func duration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("ignoring invalid duration", "key", key, "value", v)
		return fallback
	}
	return d
}

func boolEnv(key string, fallback bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}
