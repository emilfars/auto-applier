package ingest

import (
	"context"
	"sort"
	"sync"
	"time"
)

// storedJob is a job plus ingestion lifecycle metadata (last-seen + staleness).
type storedJob struct {
	job      Job
	lastSeen time.Time
	stale    bool
	staleAt  *time.Time
}

// MemoryStore is an in-memory JobStore used for tests and local development.
// Jobs are keyed by dedup_key so cross-source duplicates collapse (SCR-3). It
// tracks last-seen time so SweepStale can mark removed listings (SCR-4).
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]storedJob
	now  func() time.Time
}

// NewMemoryStore returns an empty in-memory job store using the wall clock.
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreClock(time.Now)
}

// NewMemoryStoreClock returns a store with an injected clock (for staleness
// tests). A nil clock falls back to time.Now.
func NewMemoryStoreClock(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{jobs: map[string]storedJob{}, now: now}
}

// Upsert inserts or replaces a job by its dedup key. Re-seeing a listing
// refreshes its last-seen time and clears any stale flag.
func (m *MemoryStore) Upsert(_ context.Context, j Job) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, existed := m.jobs[j.DedupKey]
	m.jobs[j.DedupKey] = storedJob{job: j, lastSeen: m.now(), stale: false, staleAt: nil}
	return !existed, nil
}

// SweepStale marks jobs not seen within maxAge as stale (SCR-4) and returns the
// number newly marked. Already-stale jobs are not recounted.
func (m *MemoryStore) SweepStale(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	marked := 0
	for key, sj := range m.jobs {
		if !sj.stale && IsStale(sj.lastSeen, now, maxAge) {
			sj.stale = true
			at := now
			sj.staleAt = &at
			m.jobs[key] = sj
			marked++
		}
	}
	return marked
}

// Len returns the number of distinct stored jobs (including stale).
func (m *MemoryStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.jobs)
}

// IsStaleKey reports whether the job with the given dedup key is marked stale.
func (m *MemoryStore) IsStaleKey(dedupKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[dedupKey].stale
}

// All returns every stored job (including stale), ordered deterministically.
func (m *MemoryStore) All() []Job {
	return m.snapshot(false)
}

// Active returns only non-stale jobs — the set the feed should surface.
func (m *MemoryStore) Active() []Job {
	return m.snapshot(true)
}

// ActiveJobs adapts the store to the feed's Provider interface.
func (m *MemoryStore) ActiveJobs(context.Context) ([]Job, error) {
	return m.Active(), nil
}

func (m *MemoryStore) snapshot(activeOnly bool) []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Job, 0, len(m.jobs))
	for _, sj := range m.jobs {
		if activeOnly && sj.stale {
			continue
		}
		out = append(out, sj.job)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Title < out[j].Title
	})
	return out
}
