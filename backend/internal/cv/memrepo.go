package cv

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryRepo is an in-memory Repo for local development and tests.
type MemoryRepo struct {
	mu    sync.Mutex
	files map[string]File // id -> file
}

// NewMemoryRepo returns an empty in-memory repo.
func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{files: make(map[string]File)}
}

func (m *MemoryRepo) CreateFile(_ context.Context, f File) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f.ID = uuid.NewString()
	if f.Label == "" {
		f.Label = f.Filename
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	if !f.IsPrimary {
		f.IsPrimary = true
		for _, existing := range m.files {
			if existing.UserID == f.UserID && existing.IsPrimary {
				f.IsPrimary = false
				break
			}
		}
	}
	if f.IsPrimary {
		for id, existing := range m.files {
			if existing.UserID == f.UserID {
				existing.IsPrimary = false
				m.files[id] = existing
			}
		}
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsPrimary != out[j].IsPrimary {
			return out[i].IsPrimary
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MemoryRepo) UpdateVersion(_ context.Context, userID, id, label string, primary *bool) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[id]
	if !ok || f.UserID != userID {
		return File{}, ErrNotFound
	}
	if strings.TrimSpace(label) != "" {
		f.Label = strings.TrimSpace(label)
	}
	if primary != nil && *primary {
		f.IsPrimary = true
		for otherID, other := range m.files {
			if other.UserID == userID {
				other.IsPrimary = otherID == id
				m.files[otherID] = other
			}
		}
	} else if primary != nil {
		f.IsPrimary = false
	}
	m.files[id] = f
	return f, nil
}

var _ VersionRepo = (*MemoryRepo)(nil)

func (m *MemoryRepo) DeleteByUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, f := range m.files {
		if f.UserID == userID {
			delete(m.files, id)
		}
	}
	return nil
}
