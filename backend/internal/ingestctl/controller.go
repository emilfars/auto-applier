// Package ingestctl decides when ingestion runs. It provides a startup
// backfill that fires only when the active real-listing count is below the
// launch target, and an authenticated endpoint that triggers one ingestion
// pass on demand. It never submits job applications (Prime Directive).
package ingestctl

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Counter reports the number of active, real (non-synthetic) listings.
type Counter interface {
	RealActiveCount(ctx context.Context) (int, error)
}

// RunFunc performs exactly one ingestion pass over all configured sources.
type RunFunc func(ctx context.Context)

// Controller serializes ingestion passes and gates the startup backfill.
type Controller struct {
	counter Counter
	target  int
	run     RunFunc
	now     func() time.Time
	minGap  time.Duration

	mu      sync.Mutex
	running bool
	lastRun time.Time
}

// New builds a Controller. A target <= 0 disables the startup backfill. minGap
// throttles repeated triggers once a pass finishes; <= 0 allows a new pass as
// soon as the previous one completes.
func New(counter Counter, target int, run RunFunc, minGap time.Duration) *Controller {
	if run == nil {
		run = func(context.Context) {}
	}
	return &Controller{counter: counter, target: target, run: run, now: time.Now, minGap: minGap}
}

// StartBackfill schedules one asynchronous ingestion pass when the active real
// count is below target. It reports whether a pass was started. A nil counter
// (in-memory mode) never triggers.
func (c *Controller) StartBackfill(ctx context.Context) (bool, error) {
	if c.counter == nil {
		return false, nil
	}
	count, err := c.counter.RealActiveCount(ctx)
	if err != nil {
		return false, err
	}
	if count >= c.target {
		return false, nil
	}
	return c.Trigger(), nil
}

// Trigger starts one asynchronous ingestion pass unless a pass is already
// running or the throttle window has not elapsed. It reports whether a pass
// was started.
//
// Re-fetching is deliberately not skipped for listings already stored: the
// store refreshes each re-seen row's last-seen time so the 48h staleness sweep
// does not expire live listings (SCR-4). Avoiding source API calls therefore
// belongs to conditional/incremental source fetching, not persistence.
func (c *Controller) Trigger() bool {
	c.mu.Lock()
	if c.running || (c.minGap > 0 && !c.lastRun.IsZero() && c.now().Sub(c.lastRun) < c.minGap) {
		c.mu.Unlock()
		return false
	}
	c.running = true
	c.lastRun = c.now()
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
		}()
		c.run(context.Background())
	}()
	return true
}

// Handler returns the on-demand trigger route. It is disabled when token is
// empty, so an unauthenticated caller can never drive ingestion.
func (c *Controller) Handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ingest/run", func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingestion trigger is not configured"})
			return
		}
		if !authorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid ingestion token"})
			return
		}
		if !c.Trigger() {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an ingestion run is already in progress"})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
	})
	return mux
}

func authorized(r *http.Request, token string) bool {
	supplied := strings.TrimSpace(r.Header.Get("X-Ingest-Token"))
	if supplied == "" {
		if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			supplied = strings.TrimSpace(bearer)
		}
	}
	if supplied == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) == 1
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("ingestctl: write response: %v", err)
	}
}
