package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newPgxTestRepo connects to TEST_DATABASE_URL, applies migrations, and returns
// a PgxRepo. It skips the test when no database is configured so the offline
// verify gate stays green; CI wires TEST_DATABASE_URL to an ephemeral Postgres.
func newPgxTestRepo(t *testing.T) (*PgxRepo, *pgxpool.Pool) {
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
	return NewPgxRepo(pool), pool
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s+%d@example.com", prefix, time.Now().UnixNano())
}

func TestPgxRepo_UserLifecycle(t *testing.T) {
	repo, _ := newPgxTestRepo(t)
	ctx := context.Background()
	email := uniqueEmail("lifecycle")

	u, err := repo.CreateUser(ctx, email, "pbkdf2_sha256$hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID == "" || u.Verified {
		t.Fatalf("unexpected user: %+v", u)
	}

	// duplicate email → ErrEmailTaken
	if _, err := repo.CreateUser(ctx, email, "x"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate: got %v, want ErrEmailTaken", err)
	}

	// lookups (email match is case-insensitive)
	got, err := repo.UserByEmail(ctx, email)
	if err != nil || got.ID != u.ID {
		t.Fatalf("by email: %+v err=%v", got, err)
	}
	if _, err := repo.UserByID(ctx, u.ID); err != nil {
		t.Fatalf("by id: %v", err)
	}
	if _, err := repo.UserByEmail(ctx, uniqueEmail("missing")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing email: got %v, want ErrNotFound", err)
	}

	// verification
	if err := repo.SetVerified(ctx, u.ID); err != nil {
		t.Fatalf("set verified: %v", err)
	}
	got, _ = repo.UserByID(ctx, u.ID)
	if !got.Verified {
		t.Fatal("user not verified after SetVerified")
	}

	// password update reflects on next lookup
	if err := repo.UpdatePassword(ctx, u.ID, "pbkdf2_sha256$new"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	got, _ = repo.UserByEmail(ctx, email)
	if got.PasswordHash != "pbkdf2_sha256$new" {
		t.Fatalf("password not updated: %q", got.PasswordHash)
	}
}

func TestPgxRepo_VerificationTokens(t *testing.T) {
	repo, _ := newPgxTestRepo(t)
	ctx := context.Background()
	u, _ := repo.CreateUser(ctx, uniqueEmail("verif"), "h")

	if err := repo.CreateVerification(ctx, u.ID, "vtok"); err != nil {
		t.Fatalf("create verification: %v", err)
	}
	uid, err := repo.ConsumeVerification(ctx, "vtok")
	if err != nil || uid != u.ID {
		t.Fatalf("consume: uid=%q err=%v", uid, err)
	}
	// single-use: second consume fails
	if _, err := repo.ConsumeVerification(ctx, "vtok"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reuse: got %v, want ErrInvalidToken", err)
	}
}

func TestPgxRepo_Sessions(t *testing.T) {
	repo, _ := newPgxTestRepo(t)
	ctx := context.Background()
	u, _ := repo.CreateUser(ctx, uniqueEmail("sess"), "h")

	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := repo.CreateSession(ctx, Session{Token: "stok", UserID: u.ID, ExpiresAt: exp}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	s, err := repo.SessionByToken(ctx, "stok")
	if err != nil || s.UserID != u.ID || !s.ExpiresAt.Equal(exp) {
		t.Fatalf("session: %+v err=%v", s, err)
	}
	if err := repo.DeleteSession(ctx, "stok"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := repo.SessionByToken(ctx, "stok"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestPgxRepo_ResetTokens(t *testing.T) {
	repo, _ := newPgxTestRepo(t)
	ctx := context.Background()
	u, _ := repo.CreateUser(ctx, uniqueEmail("reset"), "h")
	now := time.Now().UTC()

	// expired token is rejected and consumed
	if err := repo.CreateReset(ctx, u.ID, "expired", now.Add(-time.Minute)); err != nil {
		t.Fatalf("create expired reset: %v", err)
	}
	if _, err := repo.ConsumeReset(ctx, "expired", now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired: got %v, want ErrInvalidToken", err)
	}

	// valid token consumes once
	if err := repo.CreateReset(ctx, u.ID, "valid", now.Add(time.Hour)); err != nil {
		t.Fatalf("create reset: %v", err)
	}
	uid, err := repo.ConsumeReset(ctx, "valid", now)
	if err != nil || uid != u.ID {
		t.Fatalf("consume reset: uid=%q err=%v", uid, err)
	}
	if _, err := repo.ConsumeReset(ctx, "valid", now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reuse reset: got %v, want ErrInvalidToken", err)
	}
}

func TestPgxRepo_MigrationsIdempotent(t *testing.T) {
	_, pool := newPgxTestRepo(t)
	// Migrations already applied by the harness; a second Apply is a no-op.
	n, err := db.Apply(context.Background(), pool)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if n != 0 {
		t.Fatalf("re-apply applied %d migrations, want 0", n)
	}
}
