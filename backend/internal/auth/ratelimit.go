package auth

import (
	"sync"
	"time"
)

// rateLimiter is a fixed-window, per-key attempt limiter used to throttle auth
// endpoints (AUTH-4b). It is safe for concurrent use.
type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*windowState
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

// Reset clears the counter for a key (e.g. after a successful login).
func (r *rateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hits, key)
}
