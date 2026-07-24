package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

func newTestStore(t *testing.T) (*EncryptedStore, *MemoryStore) {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("gen key: %v", err)
	}
	backend := NewMemoryStore()
	enc, err := NewEncryptedStore(backend, key)
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	return enc, backend
}

func TestEncryptedStoreRoundTrip(t *testing.T) {
	enc, _ := newTestStore(t)
	ctx := context.Background()
	plaintext := []byte("Curriculum Vitae — Dina, Depok, +62-812-0000")

	if err := enc.Put(ctx, "cv/1", plaintext); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := enc.Get(ctx, "cv/1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

// AC-CV-1b: stored object is encrypted at rest — the persisted bytes must not
// equal, nor contain, the plaintext.
func TestEncryptedAtRest(t *testing.T) {
	enc, backend := newTestStore(t)
	ctx := context.Background()
	plaintext := []byte("SECRET-PII-marker-string")

	if err := enc.Put(ctx, "cv/2", plaintext); err != nil {
		t.Fatalf("Put: %v", err)
	}

	stored, err := backend.Get(ctx, "cv/2")
	if err != nil {
		t.Fatalf("backend.Get: %v", err)
	}
	if bytes.Equal(stored, plaintext) {
		t.Fatal("at-rest bytes equal plaintext — not encrypted")
	}
	if containsPlaintext(stored, plaintext) {
		t.Fatal("at-rest bytes contain plaintext marker — leak")
	}
}

func TestGetMissingKey(t *testing.T) {
	enc, _ := newTestStore(t)
	_, err := enc.Get(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTamperDetection(t *testing.T) {
	enc, backend := newTestStore(t)
	ctx := context.Background()
	if err := enc.Put(ctx, "cv/3", []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Flip a byte in the stored ciphertext.
	stored, _ := backend.Get(ctx, "cv/3")
	stored[len(stored)-1] ^= 0xFF
	_ = backend.Put(ctx, "cv/3", stored)

	if _, err := enc.Get(ctx, "cv/3"); err == nil {
		t.Fatal("expected decryption to fail on tampered ciphertext")
	}
}

func TestNewEncryptedStoreRejectsBadKey(t *testing.T) {
	if _, err := NewEncryptedStore(NewMemoryStore(), []byte("short")); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}

func TestDelete(t *testing.T) {
	enc, _ := newTestStore(t)
	ctx := context.Background()
	if err := enc.Put(ctx, "cv/4", []byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := enc.Delete(ctx, "cv/4"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := enc.Get(ctx, "cv/4"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
