package main

import (
	"context"
	"strings"
	"testing"
)

func TestNewCVStoreRequiresS3WithPersistentDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("CV_ENCRYPTION_KEY", strings.Repeat("00", 32))

	if _, err := newCVStore(context.Background()); err == nil {
		t.Fatal("expected persistent database configuration without S3 to fail")
	}
}

func TestNewCVStoreRequiresStableKeyWithS3(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("S3_ENDPOINT", "object-storage:9000")
	t.Setenv("CV_ENCRYPTION_KEY", "")

	_, err := newCVStore(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CV_ENCRYPTION_KEY") {
		t.Fatalf("expected missing persistent encryption key error, got %v", err)
	}
}
