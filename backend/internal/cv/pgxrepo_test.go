package cv

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newPgxTestRepo connects to TEST_DATABASE_URL, applies migrations, and returns
// a PgxRepo plus a fresh user id (cv_files.user_id has an FK). Skips when no
// database is configured so the offline verify gate stays green.
func newPgxTestRepo(t *testing.T) (*PgxRepo, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := db.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	var userID string
	email := fmt.Sprintf("cv+%d@example.com", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id::text`, email,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return NewPgxRepo(pool), userID
}

func TestPgxRepo_CreateGetListDelete(t *testing.T) {
	repo, userID := newPgxTestRepo(t)
	ctx := context.Background()

	created, err := repo.CreateFile(ctx, File{
		UserID:      userID,
		ObjectKey:   "cv/" + userID + "/resume.pdf",
		Filename:    "resume.pdf",
		ContentType: "application/pdf",
		SizeBytes:   2048,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("create did not populate id/created_at: %+v", created)
	}

	got, err := repo.FileByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Filename != "resume.pdf" || got.SizeBytes != 2048 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	list, err := repo.FilesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 file for user, got %d", len(list))
	}

	if err := repo.DeleteByUser(ctx, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Idempotent: deleting again is not an error.
	if err := repo.DeleteByUser(ctx, userID); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	after, err := repo.FilesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("want 0 files after delete, got %d", len(after))
	}
}

func TestPgxRepo_FileByIDMissing(t *testing.T) {
	repo, _ := newPgxTestRepo(t)
	// A random UUID that does not exist.
	_, err := repo.FileByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
