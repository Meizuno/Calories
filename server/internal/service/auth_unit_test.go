package service

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// These exercise the parts of Auth that never touch the database: the access
// token and the input rules. A nil *db.Queries is fine here — none of the
// methods under test reach for it. Database-backed behaviour lives in
// auth_integration_test.go.
func tokenAuth(ttl time.Duration) *Auth {
	return NewAuth(nil, "test-secret-at-least-32-characters-long", ttl, time.Hour)
}

func TestSignAndParseAccess(t *testing.T) {
	a := tokenAuth(time.Minute)

	tok, err := a.SignAccess("user-123")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := a.ParseAccess(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != "user-123" {
		t.Fatalf("subject = %q, want user-123", got)
	}
}

func TestParseAccessRejects(t *testing.T) {
	a := tokenAuth(time.Minute)
	valid, err := a.SignAccess("user-123")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	t.Run("expired token", func(t *testing.T) {
		expired := tokenAuth(-time.Minute) // already past its expiry
		tok, err := expired.SignAccess("user-123")
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := a.ParseAccess(tok); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expired token accepted (err=%v)", err)
		}
	})

	t.Run("signed with a different secret", func(t *testing.T) {
		other := NewAuth(nil, "a-completely-different-secret-value-32", time.Minute, time.Hour)
		tok, err := other.SignAccess("user-123")
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := a.ParseAccess(tok); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("token from a foreign secret accepted")
		}
	})

	// The classic JWT forgery: strip the signature and claim alg "none". The
	// parser must refuse on the algorithm, before it ever trusts the claims.
	t.Run("alg=none forgery", func(t *testing.T) {
		enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
		forged := enc(`{"alg":"none","typ":"JWT"}`) + "." +
			enc(`{"sub":"admin","iss":"calories","exp":99999999999}`) + "."
		if _, err := a.ParseAccess(forged); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("alg=none token accepted")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "somebody-else",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}).SignedString([]byte("test-secret-at-least-32-characters-long"))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := a.ParseAccess(tok); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("token from another issuer accepted")
		}
	})

	t.Run("empty subject", func(t *testing.T) {
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}).SignedString([]byte("test-secret-at-least-32-characters-long"))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := a.ParseAccess(tok); !errors.Is(err, ErrInvalidToken) {
			t.Fatal("token with no subject accepted")
		}
	})

	t.Run("garbage and tampering", func(t *testing.T) {
		// Tamper mid-signature, not at the end: the final base64 character carries
		// unused padding bits, so changing it can decode to the very same bytes.
		parts := strings.Split(valid, ".")
		sig := []byte(parts[2])
		sig[0] ^= 1 // flip a bit in a character that is fully significant
		tampered := parts[0] + "." + parts[1] + "." + string(sig)

		claims := []byte(parts[1])
		claims[0] ^= 1
		swappedClaims := parts[0] + "." + string(claims) + "." + parts[2]

		cases := map[string]string{
			"empty":            "",
			"not a jwt":        "hello",
			"missing sig":      strings.Join(parts[:2], "."),
			"tampered sig":     tampered,
			"tampered claims":  swappedClaims,
			"pat looks bearer": PATPrefix + "abc",
			"only dots":        "..",
		}
		for name, tok := range cases {
			if _, err := a.ParseAccess(tok); err == nil {
				t.Errorf("%s: accepted", name)
			}
		}
	})
}

func TestCheckPassword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"too short", "short", ErrWeakPassword},
		{"exactly at the minimum", "12345678", nil},
		{"ordinary", "correcthorsebattery", nil},
		// bcrypt silently truncates past 72 bytes, so a longer one must be
		// rejected rather than quietly weakened.
		{"72 bytes is fine", strings.Repeat("a", 72), nil},
		{"73 bytes is refused", strings.Repeat("a", 73), ErrLongPassword},
		// Eight characters, but 24 bytes — the length rule counts runes, while
		// the bcrypt ceiling counts bytes.
		{"multi-byte counts runes for the minimum", strings.Repeat("ě", 8), nil},
		{"multi-byte still hits the byte ceiling", strings.Repeat("ě", 40), ErrLongPassword},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := checkPassword(c.in); !errors.Is(err, c.want) {
				t.Fatalf("checkPassword(%d chars) = %v, want %v", len([]rune(c.in)), err, c.want)
			}
		})
	}
}

func TestValidEmail(t *testing.T) {
	valid := []string{"a@b.co", "Yurii@Example.com", "first.last+tag@sub.domain.org"}
	for _, e := range valid {
		if !validEmail(e) {
			t.Errorf("%q rejected, want accepted", e)
		}
	}
	invalid := []string{"", "nope", "@b.com", "a@", "a@b", "a b@c.com", "a@b .com", "a@.com", "a@b.", "a\n@b.com"}
	for _, e := range invalid {
		if validEmail(e) {
			t.Errorf("%q accepted, want rejected", e)
		}
	}
}

func TestRandomTokenIsUnpredictable(t *testing.T) {
	seen := make(map[string]bool, 256)
	for i := 0; i < 256; i++ {
		tok, err := randomToken()
		if err != nil {
			t.Fatalf("randomToken: %v", err)
		}
		if len(tok) < 40 { // 32 random bytes, base64url
			t.Fatalf("token too short: %q", tok)
		}
		if seen[tok] {
			t.Fatal("randomToken repeated a value")
		}
		seen[tok] = true
	}
}

func TestHashTokenIsStableAndOpaque(t *testing.T) {
	raw := "cal_pat_example"
	h := hashToken(raw)
	if h == raw || strings.Contains(h, raw) {
		t.Fatal("hash leaks the raw token")
	}
	if h != hashToken(raw) {
		t.Fatal("hash is not deterministic")
	}
	if len(h) != 64 { // sha256 as hex
		t.Fatalf("hash length = %d, want 64", len(h))
	}
	if hashToken("other") == h {
		t.Fatal("distinct inputs collided")
	}
}
