package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Meizuno/calories/internal/store/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Auth owns local accounts and sessions: password and Google sign-in, the signed
// access token, and the opaque rotating refresh token. It replaces the former
// external SSO — nothing here calls out to another service.
type Auth struct {
	q          *db.Queries
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// Sentinel errors the web layer maps onto status codes. Credential failures are
// deliberately coarse: a caller must not be able to tell "no such email" from
// "wrong password".
var (
	ErrEmailTaken     = errors.New("that email is already registered")
	ErrBadCredentials = errors.New("invalid email or password")
	ErrWeakPassword   = errors.New("password must be at least 8 characters")
	ErrLongPassword   = errors.New("password must be at most 72 bytes")
	ErrBadEmail       = errors.New("enter a valid email address")
	ErrNoPassword     = errors.New("this account signs in with Google")
	ErrInvalidToken   = errors.New("invalid or expired token")
)

const (
	// ProviderGoogle is the only OAuth provider today; identities.provider stores it.
	ProviderGoogle = "google"
	// bcrypt silently truncates beyond 72 bytes, so a longer password is rejected
	// outright rather than quietly weakened.
	maxPasswordBytes = 72
	minPasswordChars = 8
)

// ReuseGrace is how long a just-rotated refresh token keeps working.
//
// The server rotates on whichever request arrives first, and a page load fires
// several at once — all still carrying the same cookie, because the browser has
// not seen the new one yet. Without this window the second request would look
// like a replay and sign the user out for simply loading a page.
//
// The cost is bounded and deliberate: a genuinely stolen token also works
// inside this window. Outside it, reuse still burns the whole family.
const ReuseGrace = 20 * time.Second

func NewAuth(q *db.Queries, secret string, accessTTL, refreshTTL time.Duration) *Auth {
	return &Auth{q: q, secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (a *Auth) AccessTTL() time.Duration  { return a.accessTTL }
func (a *Auth) RefreshTTL() time.Duration { return a.refreshTTL }

// ── accounts ────────────────────────────────────────────────────────────────

// Register creates a password account. The email is stored as typed but matched
// on its lowercased form, so Bob@x.com and bob@x.com are one account.
func (a *Auth) Register(ctx context.Context, email, password, name string) (db.User, error) {
	email = strings.TrimSpace(email)
	if !validEmail(email) {
		return db.User{}, ErrBadEmail
	}
	if err := checkPassword(password); err != nil {
		return db.User{}, err
	}
	norm := strings.ToLower(email)
	if _, err := a.q.GetUserByEmail(ctx, norm); err == nil {
		return db.User{}, ErrEmailTaken
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, err
	}
	return a.q.CreateUser(ctx, db.CreateUserParams{
		Email: email, EmailNorm: norm, PasswordHash: ptr(string(hash)), Name: strings.TrimSpace(name),
	})
}

// Login verifies a password. A missing user is still run through a bcrypt compare
// so the response time does not reveal whether the email exists.
func (a *Auth) Login(ctx context.Context, email, password string) (db.User, error) {
	u, err := a.q.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return db.User{}, ErrBadCredentials
	}
	if err != nil {
		return db.User{}, err
	}
	if u.PasswordHash == nil {
		return db.User{}, ErrNoPassword
	}
	if bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)) != nil {
		return db.User{}, ErrBadCredentials
	}
	return u, nil
}

// UpsertGoogle resolves a Google sign-in to a local user: by linked identity
// first, then by matching email (Google has verified it, so this links rather
// than duplicating the account), otherwise a fresh passwordless user.
func (a *Auth) UpsertGoogle(ctx context.Context, sub, email, name string) (db.User, error) {
	if sub == "" {
		return db.User{}, ErrInvalidToken
	}
	ident, err := a.q.GetIdentity(ctx, db.GetIdentityParams{Provider: ProviderGoogle, ProviderUserID: sub})
	if err == nil {
		return a.q.GetUser(ctx, ident.UserID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, err
	}

	norm := strings.ToLower(strings.TrimSpace(email))
	u, err := a.q.GetUserByEmail(ctx, norm)
	if errors.Is(err, pgx.ErrNoRows) {
		u, err = a.q.CreateUser(ctx, db.CreateUserParams{
			Email: strings.TrimSpace(email), EmailNorm: norm, PasswordHash: nil, Name: strings.TrimSpace(name),
		})
	}
	if err != nil {
		return db.User{}, err
	}
	if _, err := a.q.LinkIdentity(ctx, db.LinkIdentityParams{
		UserID: u.ID, Provider: ProviderGoogle, ProviderUserID: sub,
	}); err != nil {
		return db.User{}, err
	}
	return u, nil
}

// SetPassword gives a Google-only account a password, or changes an existing one.
func (a *Auth) SetPassword(ctx context.Context, userID, password string) error {
	if err := checkPassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: userID, PasswordHash: ptr(string(hash))})
}

func (a *Auth) GetUser(ctx context.Context, id string) (db.User, error) { return a.q.GetUser(ctx, id) }

// Providers lists the OAuth providers linked to the account, so the profile page
// can show what it can sign in with.
func (a *Auth) Providers(ctx context.Context, userID string) ([]string, error) {
	ids, err := a.q.ListIdentities(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ids))
	for _, i := range ids {
		out = append(out, i.Provider)
	}
	return out, nil
}

// ── access token (JWT) ──────────────────────────────────────────────────────

// SignAccess mints the short-lived bearer token the API trusts on every request.
// It is stateless on purpose: no DB read per request. Revocation therefore takes
// effect within accessTTL, which is why that window is kept short.
func (a *Auth) SignAccess(userID string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    jwtIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTTL)),
	}).SignedString(a.secret)
}

// ParseAccess returns the user id carried by a valid, unexpired access token.
func (a *Auth) ParseAccess(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return a.secret, nil
	}, jwt.WithIssuer(jwtIssuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// ── refresh token (opaque, rotating) ────────────────────────────────────────

// IssueRefresh mints a new refresh token. Pass the previous token's family to
// continue a rotation chain; empty starts a new one (a fresh login).
func (a *Auth) IssueRefresh(ctx context.Context, userID, family, userAgent string) (string, error) {
	raw, err := randomToken()
	if err != nil {
		return "", err
	}
	var fam *string
	if family != "" {
		fam = &family
	}
	if _, err := a.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hashToken(raw),
		Family:    fam,
		UserAgent: trunc(userAgent, 255),
		ExpiresAt: time.Now().Add(a.refreshTTL),
	}); err != nil {
		return "", err
	}
	return raw, nil
}

// Rotate exchanges a refresh token for a fresh one. The presented token is marked
// used by a statement that also checks it was unused, so two concurrent refreshes
// cannot both succeed. A token presented twice means it leaked: the whole family
// is revoked, logging the attacker and the victim out together.
func (a *Auth) Rotate(ctx context.Context, raw, userAgent string) (db.User, string, error) {
	hash := hashToken(raw)
	rt, err := a.q.GetRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, "", ErrInvalidToken
	}
	if err != nil {
		return db.User{}, "", err
	}
	// Revoked or expired is dead regardless of anything below — checked first so
	// the grace window can never resurrect a signed-out session.
	if rt.RevokedAt.Valid || time.Now().After(rt.ExpiresAt) {
		return db.User{}, "", ErrInvalidToken
	}

	if rt.UsedAt.Valid {
		if time.Since(rt.UsedAt.Time) > ReuseGrace {
			// A genuine replay — burn the chain.
			_ = a.q.RevokeRefreshFamily(ctx, rt.Family)
			return db.User{}, "", ErrInvalidToken
		}
		return a.reissue(ctx, rt, userAgent)
	}

	n, err := a.q.UseRefreshToken(ctx, rt.ID)
	if err != nil {
		return db.User{}, "", err
	}
	if n == 0 {
		// Something changed between the read and the update. Re-read to find out
		// what: a sibling request winning the race is benign, revocation is not.
		return a.afterLostRace(ctx, hash, userAgent)
	}
	return a.reissue(ctx, rt, userAgent)
}

// afterLostRace decides whether losing the rotation race was a concurrent
// refresh (fine — hand out a sibling) or a revocation that landed in between
// (not fine — the session is over).
func (a *Auth) afterLostRace(ctx context.Context, hash, userAgent string) (db.User, string, error) {
	rt, err := a.q.GetRefreshToken(ctx, hash)
	if err != nil {
		return db.User{}, "", ErrInvalidToken
	}
	if rt.RevokedAt.Valid || time.Now().After(rt.ExpiresAt) {
		return db.User{}, "", ErrInvalidToken
	}
	if rt.UsedAt.Valid && time.Since(rt.UsedAt.Time) <= ReuseGrace {
		return a.reissue(ctx, rt, userAgent)
	}
	return db.User{}, "", ErrInvalidToken
}

// reissue mints the next token in an existing rotation chain.
func (a *Auth) reissue(ctx context.Context, rt db.RefreshToken, userAgent string) (db.User, string, error) {
	u, err := a.q.GetUser(ctx, rt.UserID)
	if err != nil {
		return db.User{}, "", err
	}
	next, err := a.IssueRefresh(ctx, u.ID, rt.Family, userAgent)
	if err != nil {
		return db.User{}, "", err
	}
	return u, next, nil
}

// RevokeRefresh ends the session the token belongs to (sign-out on this device).
func (a *Auth) RevokeRefresh(ctx context.Context, raw string) {
	rt, err := a.q.GetRefreshToken(ctx, hashToken(raw))
	if err != nil {
		return
	}
	_ = a.q.RevokeRefreshFamily(ctx, rt.Family)
}

// RevokeAll signs the user out everywhere (used after a password change).
func (a *Auth) RevokeAll(ctx context.Context, userID string) error {
	return a.q.RevokeUserRefreshTokens(ctx, userID)
}

// PurgeExpired drops refresh rows long past expiry. Called periodically by the
// server so the table does not grow without bound.
func (a *Auth) PurgeExpired(ctx context.Context) error {
	return a.q.DeleteExpiredRefreshTokens(ctx)
}

// ── helpers ─────────────────────────────────────────────────────────────────

const jwtIssuer = "calories"

// A valid bcrypt hash of an arbitrary string, compared against when the email is
// unknown so that path costs the same as a real check.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func checkPassword(p string) error {
	if len([]rune(p)) < minPasswordChars {
		return ErrWeakPassword
	}
	if len(p) > maxPasswordBytes {
		return ErrLongPassword
	}
	return nil
}

// validEmail is a deliberately loose shape check — the real proof that an address
// works is mail delivery, which this app does not do.
func validEmail(e string) bool {
	at := strings.IndexByte(e, '@')
	if at <= 0 || at == len(e)-1 || strings.ContainsAny(e, " \t\r\n") {
		return false
	}
	host := e[at+1:]
	return strings.Contains(host, ".") && !strings.HasPrefix(host, ".") && !strings.HasSuffix(host, ".")
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Refresh tokens are stored as the same sha256 hex digest as PATs — hashToken
// lives in tokens.go.

func ptr(s string) *string { return &s }

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
