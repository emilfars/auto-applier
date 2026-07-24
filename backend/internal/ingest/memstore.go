package ingest

import (
	"context"
	"sort"
	"sync"
)

// MemoryStore is an in-memory JobStore used for tests and local development.
// Jobs are keyed by dedup_key so cross-source duplicates collapse (SCR-3).
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

// NewMemoryStore returns an empty in-memory job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: map[string]Job{}}
}

// Upsert inserts or replaces a job by its dedup key.
func (m *MemoryStore) Upsert(_ context.Context, j Job) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, existed := m.jobs[j.DedupKey]
	m.jobs[j.DedupKey] = j
	return !existed, nil
}

// Len returns the number of distinct stored jobs.
func (m *MemoryStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.jobs)
}

// All returns every stored job, ordered by source then title for determinism.
func (m *MemoryStore) All() []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Title < out[j].Title
	})
	return out
}
