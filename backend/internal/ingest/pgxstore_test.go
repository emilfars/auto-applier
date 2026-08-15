package ingest

import (
	"context"
	"errors"
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
	if got.Synthetic {
		t.Fatal("real listing should remain non-synthetic")
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
	if n, err := s.RealActiveCount(ctx); err != nil || n != 1 {
		t.Fatalf("active count before revive = %d, err=%v; want 1", n, err)
	}

	// Re-seeing the stale listing clears the stale flag.
	if _, err := s.Upsert(ctx, sampleJob("old", "Old Role")); err != nil {
		t.Fatalf("re-upsert old: %v", err)
	}
	n, err := s.RealActiveCount(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 active jobs after revive, got %d", n)
	}
}

func TestPgxStore_RealActiveCountExcludesSyntheticAndStale(t *testing.T) {
	s := newPgxTestStore(t)
	ctx := context.Background()

	real := sampleJob("real", "Real Role")
	synthetic := sampleJob("synthetic", "Synthetic Role")
	synthetic.Synthetic = true
	stale := sampleJob("stale", "Stale Role")
	for _, job := range []Job{real, synthetic, stale} {
		if _, err := s.Upsert(ctx, job); err != nil {
			t.Fatalf("upsert %s: %v", job.DedupKey, err)
		}
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE jobs SET updated_at = now() - interval '72 hours' WHERE dedup_key = 'stale'`,
	); err != nil {
		t.Fatalf("age stale row: %v", err)
	}
	if _, err := s.SweepStale(ctx, StaleAfter); err != nil {
		t.Fatalf("sweep stale row: %v", err)
	}

	count, err := s.RealActiveCount(ctx)
	if err != nil {
		t.Fatalf("real active count: %v", err)
	}
	if count != 1 {
		t.Fatalf("real active count = %d, want 1", count)
	}

	active, err := s.ActiveJobs(ctx)
	if err != nil {
		t.Fatalf("active jobs: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("active jobs = %d, want real plus synthetic", len(active))
	}
	flags := make(map[string]bool, len(active))
	for _, job := range active {
		flags[job.DedupKey] = job.Synthetic
	}
	if flags["real"] || !flags["synthetic"] {
		t.Fatalf("provenance round-trip = %#v", flags)
	}

	page, err := s.SearchFeed(ctx, JobQuery{Limit: 10})
	if err != nil {
		t.Fatalf("search active jobs: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("search total = %d, want 2 active jobs", page.Total)
	}
	searchFlags := make(map[string]bool, len(page.Jobs))
	for _, job := range page.Jobs {
		searchFlags[job.DedupKey] = job.Synthetic
	}
	if searchFlags["real"] || !searchFlags["synthetic"] {
		t.Fatalf("search provenance round-trip = %#v", searchFlags)
	}
}

func TestPgxStoreRejectsUnsafeSourceURLBeforePersistence(t *testing.T) {
	store := NewPgxStore(nil)
	_, err := store.Upsert(context.Background(), Job{SourceURL: "javascript:alert(1)"})
	if !errors.Is(err, ErrInvalidSourceURL) {
		t.Fatalf("Upsert error = %v, want ErrInvalidSourceURL", err)
	}
}

func TestPgxStoreRejectsInvalidQueryBeforeDatabase(t *testing.T) {
	payMin := int64(-1)
	_, err := NewPgxStore(nil).SearchFeed(context.Background(), JobQuery{PayMin: &payMin})
	if !errors.Is(err, ErrInvalidJobQuery) {
		t.Fatalf("SearchFeed error = %v, want ErrInvalidJobQuery", err)
	}
}
