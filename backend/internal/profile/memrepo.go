package profile

import (
	"context"
	"sync"
)

// MemoryRepo is an in-memory Repo for local development and tests.
type MemoryRepo struct {
	mu       sync.Mutex
	profiles map[string]Profile // userID -> profile
}

// NewMemoryRepo returns an empty in-memory repo.
func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{profiles: make(map[string]Profile)}
}

func (m *MemoryRepo) Get(_ context.Context, userID string) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[userID]
	if !ok {
		return Profile{}, ErrNotFound
	}
	return p, nil
}

func (m *MemoryRepo) Save(_ context.Context, p Profile) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[p.UserID] = p
	return p, nil
}
