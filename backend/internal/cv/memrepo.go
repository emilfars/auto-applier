package cv

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MemoryRepo is an in-memory Repo for local development and tests.
type MemoryRepo struct {
	mu    sync.Mutex
	seq   int
	files map[string]File // id -> file
}

// NewMemoryRepo returns an empty in-memory repo.
func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{files: make(map[string]File)}
}

func (m *MemoryRepo) CreateFile(_ context.Context, f File) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	f.ID = "cv" + strconv.Itoa(m.seq)
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	m.files[f.ID] = f
	return f, nil
}

func (m *MemoryRepo) FileByID(_ context.Context, id string) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[id]
	if !ok {
		return File{}, ErrNotFound
	}
	return f, nil
}

func (m *MemoryRepo) FilesByUser(_ context.Context, userID string) ([]File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []File
	for _, f := range m.files {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	return out, nil
}
