// Package storage provides object storage for CV files with encryption at rest.
//
// Security invariant (AC-CV-1b / AC-NFR-SEC): CV bytes are AES-256-GCM encrypted
// before they ever reach the underlying object store, so the persisted bytes are
// never equal to the plaintext the user uploaded.
package storage

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrNotFound is returned when an object key does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ObjectStore is a minimal key/blob store. Implementations must be safe for
// concurrent use.
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// MemoryStore is an in-memory ObjectStore for local development and tests.
// It stores whatever bytes it is given verbatim (i.e. ciphertext when wrapped
// by EncryptedStore).
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string][]byte)}
}

func (m *MemoryStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := make([]byte, len(data))
	copy(buf, data)
	m.objects[key] = buf
	return nil
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	buf := make([]byte, len(data))
	copy(buf, data)
	return buf, nil
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// EncryptedStore wraps an ObjectStore, transparently AES-256-GCM encrypting on
// Put and decrypting on Get. The nonce is prepended to each stored object.
type EncryptedStore struct {
	backend ObjectStore
	gcm     cipher.AEAD
}

// NewEncryptedStore wraps backend with AES-256-GCM. key must be exactly 32 bytes.
func NewEncryptedStore(backend ObjectStore, key []byte) (*EncryptedStore, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("storage: encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("storage: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("storage: new gcm: %w", err)
	}
	return &EncryptedStore{backend: backend, gcm: gcm}, nil
}

func (e *EncryptedStore) Put(ctx context.Context, key string, data []byte) error {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("storage: read nonce: %w", err)
	}
	ciphertext := e.gcm.Seal(nonce, nonce, data, nil)
	return e.backend.Put(ctx, key, ciphertext)
}

func (e *EncryptedStore) Get(ctx context.Context, key string) ([]byte, error) {
	stored, err := e.backend.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	ns := e.gcm.NonceSize()
	if len(stored) < ns {
		return nil, fmt.Errorf("storage: ciphertext too short for key %q", key)
	}
	nonce, ciphertext := stored[:ns], stored[ns:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: decrypt key %q: %w", key, err)
	}
	return plaintext, nil
}

func (e *EncryptedStore) Delete(ctx context.Context, key string) error {
	return e.backend.Delete(ctx, key)
}

// containsPlaintext reports whether haystack contains needle (used by tests to
// assert at-rest bytes do not leak plaintext).
func containsPlaintext(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
