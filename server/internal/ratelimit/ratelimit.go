// Package ratelimit is a small in-memory token bucket, keyed by caller.
//
// In memory because this app runs as a single instance: a shared store would
// add an operational dependency to protect one process. If it is ever scaled
// horizontally this becomes per-instance — still useful, but the limits
// effectively multiply by the instance count, which is the moment to move the
// state to Redis.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	seen   time.Time
}

// Limiter allows at most `burst` requests per key in any `window`, refilled
// continuously rather than in steps — so a caller who stops for a third of the
// window gets a third of their allowance back, instead of everyone being let
// through at once on a window boundary.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	capacity float64       // burst
	refill   float64       // tokens per second
	ttl      time.Duration // how long an idle bucket is kept

	// now is injectable so the tests can move time without sleeping.
	now    func() time.Time
	lastGC time.Time
}

// New allows `burst` requests per `window` for each key.
func New(burst int, window time.Duration) *Limiter {
	if burst < 1 {
		burst = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{
		buckets:  make(map[string]*bucket),
		capacity: float64(burst),
		refill:   float64(burst) / window.Seconds(),
		// Keep an idle bucket for two windows: long enough that a caller cannot
		// reset their allowance by pausing, short enough that the map does not
		// grow without bound.
		ttl: 2 * window,
		now: time.Now,
	}
}

// Allow takes a token for `key`. When it returns false, retryAfter says how
// long until the next one is available — never zero, so a caller told to wait
// is never told to wait for no time.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.gc(now)

	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.capacity, seen: now}
		l.buckets[key] = b
	} else if elapsed := now.Sub(b.seen).Seconds(); elapsed > 0 {
		b.tokens = min(l.capacity, b.tokens+elapsed*l.refill)
	}
	b.seen = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.refill * float64(time.Second))
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

// gc drops buckets nobody has touched for a while. Called from Allow rather
// than a goroutine so the limiter needs no shutdown, and at most once per ttl
// so a burst of traffic does not pay for a sweep on every request.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < l.ttl {
		return
	}
	l.lastGC = now
	for k, b := range l.buckets {
		// A full bucket is indistinguishable from one that never existed, so
		// dropping it forgets nothing.
		if now.Sub(b.seen) > l.ttl {
			delete(l.buckets, k)
		}
	}
}

// Len reports how many keys are being tracked. For tests and diagnostics.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
