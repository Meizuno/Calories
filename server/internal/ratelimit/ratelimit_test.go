package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// clock lets a test move time deliberately instead of sleeping.
type clock struct{ t time.Time }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(burst int, window time.Duration) (*Limiter, *clock) {
	c := &clock{t: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)}
	l := New(burst, window)
	l.now = func() time.Time { return c.t }
	l.lastGC = c.t
	return l, c
}

func TestAllowsUpToBurstThenRefuses(t *testing.T) {
	l, _ := newTestLimiter(5, time.Minute)

	for i := 1; i <= 5; i++ {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("request %d refused, want allowed", i)
		}
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("the sixth request was allowed, want refused")
	}
	if retry <= 0 {
		t.Errorf("retryAfter = %v, want a positive wait", retry)
	}
}

func TestRefillsContinuously(t *testing.T) {
	l, c := newTestLimiter(10, time.Minute) // a token every 6s
	for i := 0; i < 10; i++ {
		l.Allow("ip")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("allowed while empty")
	}

	// Not quite one token's worth.
	c.advance(5 * time.Second)
	if ok, _ := l.Allow("ip"); ok {
		t.Error("allowed after 5s, want still refused")
	}
	// Now over the line.
	c.advance(2 * time.Second)
	if ok, _ := l.Allow("ip"); !ok {
		t.Error("refused after 7s, want one token back")
	}

	// A long pause refills to the cap and no further.
	c.advance(time.Hour)
	for i := 1; i <= 10; i++ {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("request %d after a long pause refused", i)
		}
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Error("allowed an 11th, want the cap to hold")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)
	for i := 0; i < 2; i++ {
		l.Allow("attacker")
	}
	if ok, _ := l.Allow("attacker"); ok {
		t.Fatal("attacker not limited")
	}
	// One caller exhausting their allowance must not touch anyone else's.
	if ok, _ := l.Allow("someone-else"); !ok {
		t.Error("an unrelated caller was refused")
	}
}

func TestIdleKeysAreForgotten(t *testing.T) {
	l, c := newTestLimiter(3, time.Minute)
	for i := 0; i < 50; i++ {
		l.Allow(string(rune('a' + i%26)))
	}
	if l.Len() == 0 {
		t.Fatal("nothing tracked")
	}
	// Past the ttl (2 windows) everything idle should be swept on the next call.
	c.advance(5 * time.Minute)
	l.Allow("someone")
	if n := l.Len(); n != 1 {
		t.Errorf("tracking %d keys after the sweep, want only the live one", n)
	}
}

func TestPausingDoesNotResetTheAllowance(t *testing.T) {
	// The sweep must not become a way to wipe your own bucket: a key that is
	// dropped comes back full, so it may only be dropped once it WOULD be full.
	l, c := newTestLimiter(4, time.Minute)
	for i := 0; i < 4; i++ {
		l.Allow("ip")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("allowed while empty")
	}
	// Just under the ttl: swept or not, four windows' worth of refill is already
	// more than the cap, so a full bucket is correct either way.
	c.advance(2 * time.Minute)
	allowed := 0
	for i := 0; i < 10; i++ {
		if ok, _ := l.Allow("ip"); ok {
			allowed++
		}
	}
	if allowed != 4 {
		t.Errorf("allowed %d after the pause, want exactly the burst of 4", allowed)
	}
}

func TestConcurrentCallersGetExactlyTheBurst(t *testing.T) {
	l, _ := newTestLimiter(50, time.Hour) // refill is negligible over the test
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Allow("shared"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Errorf("allowed %d of 200 concurrent requests, want exactly 50", allowed)
	}
}
