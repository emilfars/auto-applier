package m5

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MemoryStore struct {
	mu       sync.Mutex
	now      func() time.Time
	filters  map[string]SavedFilter
	states   map[string]JobState
	apps     map[string]Application
	snippets map[string]Snippet
}

func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{
		now: now, filters: map[string]SavedFilter{}, states: map[string]JobState{},
		apps: map[string]Application{}, snippets: map[string]Snippet{},
	}
}

func memoryKey(userID, key string) string { return userID + "\x00" + key }

func (m *MemoryStore) DeleteUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, value := range m.filters {
		if value.UserID == userID {
			delete(m.filters, id)
		}
	}
	for key := range m.states {
		if strings.HasPrefix(key, userID+"\x00") {
			delete(m.states, key)
		}
	}
	for id, value := range m.apps {
		if value.UserID == userID {
			delete(m.apps, id)
		}
	}
	for id, value := range m.snippets {
		if value.UserID == userID {
			delete(m.snippets, id)
		}
	}
	return nil
}

func (m *MemoryStore) ListSavedFilterUsers(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, filter := range m.filters {
		if !seen[filter.UserID] {
			seen[filter.UserID] = true
			out = append(out, filter.UserID)
		}
	}
	return out, nil
}

func (m *MemoryStore) ExportUser(_ context.Context, userID string) (UserData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := UserData{SavedFilters: []SavedFilter{}, JobStates: []JobState{}, Applications: []Application{}, Snippets: []Snippet{}}
	for _, filter := range m.filters {
		if filter.UserID == userID {
			data.SavedFilters = append(data.SavedFilters, filter)
		}
	}
	for key, state := range m.states {
		if strings.HasPrefix(key, userID+"\x00") {
			data.JobStates = append(data.JobStates, state)
		}
	}
	for _, app := range m.apps {
		if app.UserID == userID {
			data.Applications = append(data.Applications, app)
		}
	}
	for _, snippet := range m.snippets {
		if snippet.UserID == userID {
			data.Snippets = append(data.Snippets, snippet)
		}
	}
	return data, nil
}

func (m *MemoryStore) ListSavedFilters(_ context.Context, userID string) ([]SavedFilter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []SavedFilter
	for _, f := range m.filters {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *MemoryStore) GetSavedFilter(_ context.Context, userID, id string) (SavedFilter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.filters[id]
	if !ok || f.UserID != userID {
		return SavedFilter{}, ErrNotFound
	}
	return f, nil
}

func (m *MemoryStore) CreateSavedFilter(_ context.Context, f SavedFilter) (SavedFilter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f.Name = strings.TrimSpace(f.Name)
	for _, existing := range m.filters {
		if existing.UserID == f.UserID && strings.EqualFold(existing.Name, f.Name) {
			return SavedFilter{}, ErrConflict
		}
	}
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = m.now().UTC()
	}
	m.filters[f.ID] = f
	return f, nil
}

func (m *MemoryStore) DeleteSavedFilter(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.filters[id]
	if !ok || f.UserID != userID {
		return ErrNotFound
	}
	delete(m.filters, id)
	return nil
}

func (m *MemoryStore) TouchSavedFilter(_ context.Context, userID, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.filters[id]
	if !ok || f.UserID != userID {
		return ErrNotFound
	}
	f.LastAlertedAt = &at
	m.filters[id] = f
	return nil
}

func (m *MemoryStore) GetJobState(_ context.Context, userID, key string) (JobState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[memoryKey(userID, key)]
	if !ok {
		return JobState{JobKey: key}, nil
	}
	return state, nil
}

func (m *MemoryStore) SetJobState(_ context.Context, userID string, state JobState) (JobState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state.JobKey = strings.TrimSpace(state.JobKey)
	state.UpdatedAt = m.now().UTC()
	m.states[memoryKey(userID, state.JobKey)] = state
	return state, nil
}

func (m *MemoryStore) ListApplications(_ context.Context, userID string) ([]Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Application
	for _, app := range m.apps {
		if app.UserID == userID {
			out = append(out, app)
		}
	}
	return out, nil
}

func (m *MemoryStore) GetApplication(_ context.Context, userID, id string) (Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	app, ok := m.apps[id]
	if !ok || app.UserID != userID {
		return Application{}, ErrNotFound
	}
	return app, nil
}

func (m *MemoryStore) CreateApplication(_ context.Context, app Application) (Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.apps {
		if existing.UserID == app.UserID && existing.JobKey == app.JobKey {
			return existing, nil
		}
	}
	if app.ID == "" {
		app.ID = uuid.NewString()
	}
	if app.CreatedAt.IsZero() {
		app.CreatedAt = m.now().UTC()
	}
	if app.UpdatedAt.IsZero() {
		app.UpdatedAt = app.CreatedAt
	}
	m.apps[app.ID] = app
	return app, nil
}

func (m *MemoryStore) UpdateApplication(_ context.Context, userID string, app Application) (Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.apps[app.ID]
	if !ok || existing.UserID != userID {
		return Application{}, ErrNotFound
	}
	existing.Status = app.Status
	existing.UpdatedAt = m.now().UTC()
	m.apps[app.ID] = existing
	return existing, nil
}

func (m *MemoryStore) ListSnippets(_ context.Context, userID string) ([]Snippet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Snippet
	for _, snippet := range m.snippets {
		if snippet.UserID == userID {
			out = append(out, snippet)
		}
	}
	return out, nil
}

func (m *MemoryStore) GetSnippet(_ context.Context, userID, id string) (Snippet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snippet, ok := m.snippets[id]
	if !ok || snippet.UserID != userID {
		return Snippet{}, ErrNotFound
	}
	return snippet, nil
}

func (m *MemoryStore) CreateSnippet(_ context.Context, snippet Snippet) (Snippet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.snippets {
		if existing.UserID == snippet.UserID && strings.EqualFold(existing.Name, snippet.Name) {
			return Snippet{}, ErrConflict
		}
	}
	if snippet.ID == "" {
		snippet.ID = uuid.NewString()
	}
	now := m.now().UTC()
	if snippet.CreatedAt.IsZero() {
		snippet.CreatedAt = now
	}
	if snippet.UpdatedAt.IsZero() {
		snippet.UpdatedAt = snippet.CreatedAt
	}
	m.snippets[snippet.ID] = snippet
	return snippet, nil
}

func (m *MemoryStore) UpdateSnippet(_ context.Context, userID string, snippet Snippet) (Snippet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.snippets[snippet.ID]
	if !ok || existing.UserID != userID {
		return Snippet{}, ErrNotFound
	}
	for _, other := range m.snippets {
		if other.ID != snippet.ID && other.UserID == userID && strings.EqualFold(other.Name, snippet.Name) {
			return Snippet{}, ErrConflict
		}
	}
	existing.Name = strings.TrimSpace(snippet.Name)
	existing.Body = snippet.Body
	existing.UpdatedAt = m.now().UTC()
	m.snippets[snippet.ID] = existing
	return existing, nil
}

func (m *MemoryStore) DeleteSnippet(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	snippet, ok := m.snippets[id]
	if !ok || snippet.UserID != userID {
		return ErrNotFound
	}
	delete(m.snippets, id)
	return nil
}

var _ Store = (*MemoryStore)(nil)
