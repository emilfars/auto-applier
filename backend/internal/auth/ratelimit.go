package auth

import (
	"sync"
	"time"
)

// rateLimiter is a fixed-window, per-key attempt limiter used to throttle auth
// endpoints (AUTH-4b). It is safe for concurrent use.
//
// Expired windows are evicted lazily: once per window a sweep drops keys whose
// window has elapsed, so the map cannot grow without bound from distinct keys
// (e.g. spraying rotating emails/IPs) — guarding against memory-exhaustion DoS.
type rateLimiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	hits      map[string]*windowState
	nextSweep time.Time
}

type windowState struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, hits: make(map[string]*windowState)}
}

// Allow records an attempt for key and reports whether it is within the limit.
// The (max+1)-th attempt inside a window returns false.
func (r *rateLimiter) Allow(key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweep(now)
	st, ok := r.hits[key]
	if !ok || now.After(st.resetAt) {
		r.hits[key] = &windowState{count: 1, resetAt: now.Add(r.window)}
		return true
	}
	if st.count >= r.max {
		return false
	}
	st.count++
	return true
}

// sweep evicts expired windows. It runs at most once per window to keep the
// amortized cost negligible. Caller must hold r.mu.
func (r *rateLimiter) sweep(now time.Time) {
	if now.Before(r.nextSweep) {
		return
	}
	for k, st := range r.hits {
		if now.After(st.resetAt) {
			delete(r.hits, k)
		}
	}
	r.nextSweep = now.Add(r.window)
}

// Reset clears the counter for a key (e.g. after a successful login).
func (r *rateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hits, key)
}

// size reports the number of tracked keys. Used in tests to assert eviction.
func (r *rateLimiter) size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.hits)
}
