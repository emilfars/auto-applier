package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newPgxTestRepo connects to TEST_DATABASE_URL, applies migrations, and returns
// a PgxRepo plus a freshly-created user id (profiles.user_id has an FK). Skips
// when no database is configured so the offline verify gate stays green.
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
	email := fmt.Sprintf("profile+%d@example.com", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id::text`, email,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return NewPgxRepo(pool), userID
}

func TestPgxRepo_GetMissing(t *testing.T) {
	repo, userID := newPgxTestRepo(t)
	if _, err := repo.Get(context.Background(), userID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing: got %v, want ErrNotFound", err)
	}
}

func TestPgxRepo_SaveAndUpdate(t *testing.T) {
	repo, userID := newPgxTestRepo(t)
	ctx := context.Background()

	salary := int64(15000000)
	notice := 30
	p := emptyProfile(userID)
	p.FullName = "Dina Putri"
	p.Skills = json.RawMessage(`["Go","React"]`)
	p.ExpectedSalary = &salary
	p.NoticePeriodDays = &notice
	p.EmploymentType = "full_time"

	saved, err := repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.FullName != "Dina Putri" {
		t.Fatalf("unexpected saved profile: %+v", saved)
	}
	var skills []string
	if err := json.Unmarshal(saved.Skills, &skills); err != nil || len(skills) != 2 {
		t.Fatalf("skills round-trip failed: %s (%v)", saved.Skills, err)
	}

	got, err := repo.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ExpectedSalary == nil || *got.ExpectedSalary != salary {
		t.Fatalf("expected_salary round-trip failed: %+v", got.ExpectedSalary)
	}

	// update: confirm the profile
	now := time.Now().UTC().Truncate(time.Second)
	got.Confirmed = true
	got.ConfirmedAt = &now
	if _, err := repo.Save(ctx, got); err != nil {
		t.Fatalf("update save: %v", err)
	}
	after, _ := repo.Get(ctx, userID)
	if !after.Confirmed || after.ConfirmedAt == nil {
		t.Fatalf("confirm not persisted: %+v", after)
	}
	if !after.CanArm() {
		t.Fatal("CanArm should be true after confirm")
	}
}
