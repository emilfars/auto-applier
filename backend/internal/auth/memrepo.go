package auth

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MemoryRepo is an in-memory Repo for local development and tests.
type MemoryRepo struct {
	mu       sync.Mutex
	seq      int
	users    map[string]User        // id -> user
	byEmail  map[string]string      // lower(email) -> id
	verifs   map[string]string      // token -> userID
	sessions map[string]Session     // token -> session
	resets   map[string]resetRecord // token -> record
}

type resetRecord struct {
	userID    string
	expiresAt time.Time
}

// NewMemoryRepo returns an empty in-memory repo.
func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{
		users:    make(map[string]User),
		byEmail:  make(map[string]string),
		verifs:   make(map[string]string),
		sessions: make(map[string]Session),
		resets:   make(map[string]resetRecord),
	}
}

func (m *MemoryRepo) CreateUser(_ context.Context, email, passwordHash string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(email)
	if _, ok := m.byEmail[key]; ok {
		return User{}, ErrEmailTaken
	}
	m.seq++
	u := User{
		ID:           "u" + strconv.Itoa(m.seq),
		Email:        email,
		PasswordHash: passwordHash,
		Verified:     false,
		CreatedAt:    time.Now(),
	}
	m.users[u.ID] = u
	m.byEmail[key] = u.ID
	return u, nil
}

func (m *MemoryRepo) UserByEmail(_ context.Context, email string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byEmail[strings.ToLower(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return m.users[id], nil
}

func (m *MemoryRepo) UserByID(_ context.Context, id string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MemoryRepo) SetVerified(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.Verified = true
	m.users[userID] = u
	return nil
}

func (m *MemoryRepo) UpdatePassword(_ context.Context, userID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.PasswordHash = passwordHash
	m.users[userID] = u
	return nil
}

func (m *MemoryRepo) CreateVerification(_ context.Context, userID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifs[token] = userID
	return nil
}

func (m *MemoryRepo) ConsumeVerification(_ context.Context, token string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	userID, ok := m.verifs[token]
	if !ok {
		return "", ErrInvalidToken
	}
	delete(m.verifs, token)
	return userID, nil
}

func (m *MemoryRepo) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.Token] = s
	return nil
}

func (m *MemoryRepo) SessionByToken(_ context.Context, token string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (m *MemoryRepo) DeleteSession(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

func (m *MemoryRepo) CreateReset(_ context.Context, userID, token string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resets[token] = resetRecord{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *MemoryRepo) ConsumeReset(_ context.Context, token string, now time.Time) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.resets[token]
	if !ok {
		return "", ErrInvalidToken
	}
	if now.After(rec.expiresAt) {
		delete(m.resets, token)
		return "", ErrInvalidToken
	}
	delete(m.resets, token)
	return rec.userID, nil
}

// VerificationToken returns the outstanding verification token for a user
// (test/dev helper — production emails the token instead).
func (m *MemoryRepo) VerificationToken(userID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, uid := range m.verifs {
		if uid == userID {
			return tok, true
		}
	}
	return "", false
}

// ResetToken returns the outstanding password-reset token for a user
// (test/dev helper — production emails the token instead).
func (m *MemoryRepo) ResetToken(userID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok, rec := range m.resets {
		if rec.userID == userID {
			return tok, true
		}
	}
	return "", false
}
