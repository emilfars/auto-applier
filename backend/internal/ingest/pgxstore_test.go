package ingest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newPgxTestStore connects to TEST_DATABASE_URL, applies migrations, and returns
// a PgxStore over a clean jobs table. Skips when no database is configured so
// the offline verify gate stays green.
func newPgxTestStore(t *testing.T) *PgxStore {
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
	if _, err := pool.Exec(ctx, `TRUNCATE jobs CASCADE`); err != nil {
		t.Fatalf("truncate jobs: %v", err)
	}
	return NewPgxStore(pool)
}

func ptrI64(v int64) *int64 { return &v }
func ptrInt(v int) *int     { return &v }

func sampleJob(dedup, title string) Job {
	posted := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	return Job{
		Source:          "kalibrr",
		SourceURL:       "https://example.test/" + dedup,
		DedupKey:        dedup,
		Title:           title,
		Company:         "Nusantara Digital",
		Location:        "Jakarta Selatan, DKI Jakarta",
		Remote:          false,
		SalaryStatedMin: ptrI64(8_000_000),
		SalaryStatedMax: ptrI64(12_000_000),
		SalaryCurrency:  "IDR",
		Seniority:       "mid",
		EmploymentType:  "full_time",
		YearsExperience: ptrInt(3),
		Requirements:    []string{"Go", "PostgreSQL"},
		PostedAt:        &posted,
	}
}

func TestPgxStore_UpsertCreatesThenUpdates(t *testing.T) {
	s := newPgxTestStore(t)
	ctx := context.Background()

	created, err := s.Upsert(ctx, sampleJob("k1", "Backend Engineer"))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created {
		t.Fatal("first upsert should report created=true")
	}

	// Same dedup key with a changed title → update, not create.
	created, err = s.Upsert(ctx, sampleJob("k1", "Senior Backend Engineer"))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created {
		t.Fatal("second upsert should report created=false (update)")
	}

	jobs, err := s.ActiveJobs(ctx)
	if err != nil {
		t.Fatalf("active jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("want 1 job after dedup, got %d", len(jobs))
	}
	got := jobs[0]
	if got.Title != "Senior Backend Engineer" {
		t.Fatalf("update not applied: %q", got.Title)
	}
	if got.SalaryStatedMin == nil || *got.SalaryStatedMin != 8_000_000 {
		t.Fatalf("salary round-trip failed: %+v", got.SalaryStatedMin)
	}
	if len(got.Requirements) != 2 || got.Requirements[0] != "Go" {
		t.Fatalf("requirements round-trip failed: %#v", got.Requirements)
	}
}

func TestPgxStore_ActiveExcludesStale(t *testing.T) {
	s := newPgxTestStore(t)
	ctx := context.Background()

	if _, err := s.Upsert(ctx, sampleJob("fresh", "Fresh Role")); err != nil {
		t.Fatalf("upsert fresh: %v", err)
	}
	if _, err := s.Upsert(ctx, sampleJob("old", "Old Role")); err != nil {
		t.Fatalf("upsert old: %v", err)
	}
	// Age the "old" row beyond the staleness window.
	if _, err := s.db.Exec(ctx,
		`UPDATE jobs SET updated_at = now() - interval '72 hours' WHERE dedup_key = 'old'`,
	); err != nil {
		t.Fatalf("age old row: %v", err)
	}

	marked, err := s.SweepStale(ctx, 48*time.Hour)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if marked != 1 {
		t.Fatalf("want 1 job marked stale, got %d", marked)
	}

	jobs, err := s.ActiveJobs(ctx)
	if err != nil {
		t.Fatalf("active jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].DedupKey != "fresh" {
		t.Fatalf("stale job leaked into feed: %#v", jobs)
	}

	// Re-seeing the stale listing clears the stale flag.
	if _, err := s.Upsert(ctx, sampleJob("old", "Old Role")); err != nil {
		t.Fatalf("re-upsert old: %v", err)
	}
	n, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 active jobs after revive, got %d", n)
	}
}
