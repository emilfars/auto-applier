// Package cv handles CV file uploads. Uploaded bytes are encrypted at rest via
// the storage layer (AC-CV-1b); Postgres holds only object-store references and
// metadata, never CV contents/PII.
package cv

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a CV file record does not exist.
var ErrNotFound = errors.New("cv: file not found")

// File is a stored CV file's metadata record.
type File struct {
	ID          string
	UserID      string
	ObjectKey   string
	Filename    string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

// Repo persists CV file metadata. Implementations may be in-memory (tests/dev)
// or Postgres-backed (production).
type Repo interface {
	CreateFile(ctx context.Context, f File) (File, error)
	FileByID(ctx context.Context, id string) (File, error)
	FilesByUser(ctx context.Context, userID string) ([]File, error)
	// DeleteByUser removes all of a user's CV metadata rows. Idempotent
	// (right to erasure, AC-AUTH-5). Callers must delete the referenced
	// objects from storage separately.
	DeleteByUser(ctx context.Context, userID string) error
}
