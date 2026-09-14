package service_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
	"github.com/Meizuno/calories/internal/store/db"
)

// Behaviour that only exists against a real database: rotation, reuse
// detection, account linking. These skip unless TEST_DATABASE_URL points at a
// throwaway database — they TRUNCATE, so never aim it at anything real.
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/calories_test?sslmode=disable go test ./...
func testStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping database-backed tests")
	}
	if err := store.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := store.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(st.Close)

	// Start each test from an empty slate. profiles/identities/refresh_tokens all
	// cascade from users, but naming them keeps this honest if that ever changes.
	if _, err := st.Pool.Exec(context.Background(),
		`TRUNCATE users, profiles, identities, refresh_tokens RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return st
}

func newAuth(t *testing.T) (*service.Auth, *store.Store) {
	t.Helper()
	st := testStore(t)
	return service.NewAuth(st.Queries, "test-secret-at-least-32-characters-long", time.Minute, time.Hour), st
}

func mustRegister(t *testing.T, a *service.Auth, email, password string) db.User {
	t.Helper()
	u, err := a.Register(context.Background(), email, password, "Test")
	if err != nil {
		t.Fatalf("register %s: %v", email, err)
	}
	return u
}

func TestRegister(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()

	t.Run("stores a hash, never the password", func(t *testing.T) {
		u := mustRegister(t, a, "hash@example.com", "correcthorse")
		if u.PasswordHash == nil {
			t.Fatal("no password hash stored")
		}
		if strings.Contains(*u.PasswordHash, "correcthorse") {
			t.Fatal("the password itself is in the stored hash")
		}
		if !strings.HasPrefix(*u.PasswordHash, "$2") {
			t.Fatalf("not a bcrypt hash: %q", *u.PasswordHash)
		}
	})

	t.Run("email is stored as typed but matched lowercased", func(t *testing.T) {
		u := mustRegister(t, a, "Mixed@Example.com", "correcthorse")
		if u.Email != "Mixed@Example.com" {
			t.Fatalf("email = %q, want it preserved as typed", u.Email)
		}
		if u.EmailNorm != "mixed@example.com" {
			t.Fatalf("email_norm = %q, want lowercased", u.EmailNorm)
		}
	})

	t.Run("duplicate email is refused, regardless of case", func(t *testing.T) {
		mustRegister(t, a, "dupe@example.com", "correcthorse")
		for _, variant := range []string{"dupe@example.com", "DUPE@Example.com", "  dupe@example.com  "} {
			if _, err := a.Register(ctx, variant, "correcthorse", ""); !errors.Is(err, service.ErrEmailTaken) {
				t.Fatalf("register(%q) = %v, want ErrEmailTaken", variant, err)
			}
		}
	})

	t.Run("validation runs before anything is written", func(t *testing.T) {
		if _, err := a.Register(ctx, "not-an-email", "correcthorse", ""); !errors.Is(err, service.ErrBadEmail) {
			t.Fatalf("bad email = %v", err)
		}
		if _, err := a.Register(ctx, "short@example.com", "abc", ""); !errors.Is(err, service.ErrWeakPassword) {
			t.Fatalf("weak password = %v", err)
		}
		// The rejected address must not have been created.
		if _, err := a.Login(ctx, "short@example.com", "abc"); !errors.Is(err, service.ErrBadCredentials) {
			t.Fatalf("a rejected registration left an account behind (err=%v)", err)
		}
	})
}

func TestLogin(t *testing.T) {
	a, st := newAuth(t)
	ctx := context.Background()
	mustRegister(t, a, "login@example.com", "correcthorse")

	t.Run("correct password", func(t *testing.T) {
		if _, err := a.Login(ctx, "login@example.com", "correcthorse"); err != nil {
			t.Fatalf("login: %v", err)
		}
	})

	t.Run("email match is case and space insensitive", func(t *testing.T) {
		if _, err := a.Login(ctx, "  LOGIN@Example.com ", "correcthorse"); err != nil {
			t.Fatalf("login: %v", err)
		}
	})

	// A wrong password and an unknown address must be indistinguishable, or the
	// endpoint becomes an account-enumeration oracle.
	t.Run("wrong password and unknown email look identical", func(t *testing.T) {
		_, wrong := a.Login(ctx, "login@example.com", "not-the-password")
		_, unknown := a.Login(ctx, "ghost@example.com", "not-the-password")
		if !errors.Is(wrong, service.ErrBadCredentials) {
			t.Fatalf("wrong password = %v", wrong)
		}
		if !errors.Is(unknown, service.ErrBadCredentials) {
			t.Fatalf("unknown email = %v", unknown)
		}
		if wrong.Error() != unknown.Error() {
			t.Fatalf("messages differ: %q vs %q", wrong, unknown)
		}
	})

	t.Run("google-only account has no password to check", func(t *testing.T) {
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO users (email, email_norm, password_hash) VALUES ($1,$1,NULL)`,
			"googleonly@example.com"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if _, err := a.Login(ctx, "googleonly@example.com", "anything-at-all"); !errors.Is(err, service.ErrNoPassword) {
			t.Fatalf("login = %v, want ErrNoPassword", err)
		}
	})
}

func TestRefreshRotation(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()
	u := mustRegister(t, a, "rotate@example.com", "correcthorse")

	t.Run("rotating issues a new token and retires the old", func(t *testing.T) {
		first, err := a.IssueRefresh(ctx, u.ID, "", "test-agent")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		got, second, err := a.Rotate(ctx, first, "test-agent")
		if err != nil {
			t.Fatalf("rotate: %v", err)
		}
		if got.ID != u.ID {
			t.Fatalf("rotate returned user %q, want %q", got.ID, u.ID)
		}
		if second == first {
			t.Fatal("rotate handed back the same token")
		}
		// The new one keeps working.
		if _, _, err := a.Rotate(ctx, second, "test-agent"); err != nil {
			t.Fatalf("second rotate: %v", err)
		}
	})

	// The point of rotation: a token presented twice means it leaked, so the
	// whole chain dies — attacker and victim are signed out together.
	t.Run("replaying a spent token burns the family", func(t *testing.T) {
		first, err := a.IssueRefresh(ctx, u.ID, "", "test-agent")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		_, second, err := a.Rotate(ctx, first, "test-agent")
		if err != nil {
			t.Fatalf("rotate: %v", err)
		}

		if _, _, err := a.Rotate(ctx, first, "test-agent"); !errors.Is(err, service.ErrInvalidToken) {
			t.Fatalf("replay accepted (err=%v)", err)
		}
		// And the legitimate successor must now be dead too.
		if _, _, err := a.Rotate(ctx, second, "test-agent"); !errors.Is(err, service.ErrInvalidToken) {
			t.Fatal("the rest of the family survived a detected replay")
		}
	})

	t.Run("unknown token is refused", func(t *testing.T) {
		if _, _, err := a.Rotate(ctx, "never-issued", "test-agent"); !errors.Is(err, service.ErrInvalidToken) {
			t.Fatalf("unknown token = %v", err)
		}
	})

	// Two tabs refreshing at once must not both succeed: the UPDATE that marks a
	// token used also asserts it was unused, so exactly one wins.
	t.Run("concurrent rotation has a single winner", func(t *testing.T) {
		tok, err := a.IssueRefresh(ctx, u.ID, "", "test-agent")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		const racers = 8
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		wg.Add(racers)
		for i := 0; i < racers; i++ {
			go func() {
				defer wg.Done()
				if _, _, err := a.Rotate(context.Background(), tok, "test-agent"); err == nil {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("%d concurrent rotations succeeded, want exactly 1", wins)
		}
	})

	t.Run("revoking all ends every session", func(t *testing.T) {
		a2, _ := newAuth(t)
		u2 := mustRegister(t, a2, "revoke@example.com", "correcthorse")
		var tokens []string
		for i := 0; i < 3; i++ {
			tok, err := a2.IssueRefresh(ctx, u2.ID, "", "agent")
			if err != nil {
				t.Fatalf("issue: %v", err)
			}
			tokens = append(tokens, tok)
		}
		if err := a2.RevokeAll(ctx, u2.ID); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		for i, tok := range tokens {
			if _, _, err := a2.Rotate(ctx, tok, "agent"); !errors.Is(err, service.ErrInvalidToken) {
				t.Fatalf("token %d still worked after RevokeAll", i)
			}
		}
	})
}

func TestUpsertGoogle(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a passwordless account on first sign-in", func(t *testing.T) {
		a, _ := newAuth(t)
		u, err := a.UpsertGoogle(ctx, "google-sub-1", "New@Example.com", "New")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if u.PasswordHash != nil {
			t.Fatal("a Google account should start without a password")
		}
		if u.EmailNorm != "new@example.com" {
			t.Fatalf("email_norm = %q", u.EmailNorm)
		}
		providers, err := a.Providers(ctx, u.ID)
		if err != nil {
			t.Fatalf("providers: %v", err)
		}
		if len(providers) != 1 || providers[0] != service.ProviderGoogle {
			t.Fatalf("providers = %v", providers)
		}
	})

	t.Run("same subject returns the same account", func(t *testing.T) {
		a, _ := newAuth(t)
		first, err := a.UpsertGoogle(ctx, "google-sub-2", "same@example.com", "Same")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		// Even if the address on the Google side changed, the subject is the key.
		second, err := a.UpsertGoogle(ctx, "google-sub-2", "changed@example.com", "Same")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if first.ID != second.ID {
			t.Fatalf("same subject produced two accounts: %s and %s", first.ID, second.ID)
		}
	})

	// Linking by email is what lets one person use both doors. It is also why
	// open registration would be dangerous without email verification — see the
	// allowlist in the web layer.
	t.Run("links onto an existing password account by email", func(t *testing.T) {
		a, _ := newAuth(t)
		existing := mustRegister(t, a, "both@example.com", "correcthorse")
		linked, err := a.UpsertGoogle(ctx, "google-sub-3", "BOTH@example.com", "Both")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if linked.ID != existing.ID {
			t.Fatal("Google sign-in created a duplicate instead of linking")
		}
		if linked.PasswordHash == nil {
			t.Fatal("linking wiped the existing password")
		}
		// Both routes still work.
		if _, err := a.Login(ctx, "both@example.com", "correcthorse"); err != nil {
			t.Fatalf("password login broke after linking: %v", err)
		}
	})

	t.Run("a missing subject is refused", func(t *testing.T) {
		a, _ := newAuth(t)
		if _, err := a.UpsertGoogle(ctx, "", "x@example.com", "X"); !errors.Is(err, service.ErrInvalidToken) {
			t.Fatalf("empty subject = %v", err)
		}
	})
}

func TestSetPassword(t *testing.T) {
	a, _ := newAuth(t)
	ctx := context.Background()

	t.Run("gives a Google-only account a password", func(t *testing.T) {
		u, err := a.UpsertGoogle(ctx, "google-sub-4", "setpw@example.com", "Set")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if err := a.SetPassword(ctx, u.ID, "brandnewpassword"); err != nil {
			t.Fatalf("set: %v", err)
		}
		if _, err := a.Login(ctx, "setpw@example.com", "brandnewpassword"); err != nil {
			t.Fatalf("login after set: %v", err)
		}
	})

	t.Run("replaces an existing password", func(t *testing.T) {
		u := mustRegister(t, a, "change@example.com", "oldpassword1")
		if err := a.SetPassword(ctx, u.ID, "newpassword12"); err != nil {
			t.Fatalf("set: %v", err)
		}
		if _, err := a.Login(ctx, "change@example.com", "oldpassword1"); !errors.Is(err, service.ErrBadCredentials) {
			t.Fatal("the old password still works")
		}
		if _, err := a.Login(ctx, "change@example.com", "newpassword12"); err != nil {
			t.Fatalf("new password rejected: %v", err)
		}
	})

	t.Run("weak passwords are refused", func(t *testing.T) {
		u := mustRegister(t, a, "weak@example.com", "correcthorse")
		if err := a.SetPassword(ctx, u.ID, "abc"); !errors.Is(err, service.ErrWeakPassword) {
			t.Fatalf("set = %v, want ErrWeakPassword", err)
		}
	})
}
