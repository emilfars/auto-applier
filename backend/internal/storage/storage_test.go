package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
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

func TestNewS3StoreRejectsIncompleteConfig(t *testing.T) {
	if _, err := NewS3Store(context.Background(), S3Config{}); err == nil {
		t.Fatal("expected incomplete S3 configuration to fail")
	}
}

func TestS3StorePersistsEncryptedBytes_AC_NFR_SEC(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT not set")
	}
	ctx := context.Background()
	bucket := os.Getenv("TEST_S3_BUCKET")
	if bucket == "" {
		bucket = fmt.Sprintf("cv-test-%d", time.Now().UnixNano())
	}
	cfg := S3Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
		Bucket:    bucket,
	}
	raw, err := NewS3Store(ctx, cfg)
	if err != nil {
		t.Fatalf("create first S3 store: %v", err)
	}
	key := bytes.Repeat([]byte{0x2a}, 32)
	objectKey := "cv/1"
	plaintext := []byte("persistent CV bytes")
	if smokeKey := os.Getenv("TEST_S3_OBJECT_KEY"); smokeKey != "" {
		objectKey = smokeKey
		plaintext = []byte(os.Getenv("TEST_S3_PLAINTEXT"))
		hexKey := os.Getenv("TEST_S3_ENCRYPTION_KEY")
		var err error
		key, err = hex.DecodeString(hexKey)
		if err != nil || len(key) != 32 {
			t.Fatalf("decode smoke encryption key: invalid key")
		}
	}
	encrypted, err := NewEncryptedStore(raw, key)
	if err != nil {
		t.Fatalf("wrap first S3 store: %v", err)
	}
	if os.Getenv("TEST_S3_OBJECT_KEY") == "" {
		if err := encrypted.Put(ctx, objectKey, plaintext); err != nil {
			t.Fatalf("put encrypted object: %v", err)
		}
	}

	reopened, err := NewS3Store(ctx, cfg)
	if err != nil {
		t.Fatalf("reopen S3 store: %v", err)
	}
	stored, err := reopened.Get(ctx, objectKey)
	if err != nil {
		t.Fatalf("get stored ciphertext: %v", err)
	}
	if bytes.Equal(stored, plaintext) {
		t.Fatal("S3 persisted plaintext instead of ciphertext")
	}
	if bytes.Contains(stored, plaintext) {
		t.Fatal("S3 ciphertext contains plaintext")
	}
	decrypted, err := NewEncryptedStore(reopened, key)
	if err != nil {
		t.Fatalf("wrap reopened S3 store: %v", err)
	}
	got, err := decrypted.Get(ctx, objectKey)
	if err != nil {
		t.Fatalf("get decrypted object: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("reopened S3 round-trip mismatch: got %q want %q", got, plaintext)
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
