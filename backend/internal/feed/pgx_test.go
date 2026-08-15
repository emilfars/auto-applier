package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newFeedPgxStore(t *testing.T) (*ingest.PgxStore, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres feed integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
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
	return ingest.NewPgxStore(pool), pool
}

func TestPgxFeedCombinedFiltersPaginationAndStatedPay(t *testing.T) {
	store, pool := newFeedPgxStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	jobs := []ingest.Job{
		{
			Source: "jobstreet", SourceURL: "https://example.test/one", DedupKey: "feed-one",
			Title: "Senior Go Engineer", Company: "Acme", Location: "Jakarta Selatan",
			SalaryStatedMin: ptrI64(10_000_000), SalaryStatedMax: ptrI64(20_000_000),
			SalaryCurrency: "IDR", EmploymentType: "full_time", YearsExperience: ptrInt(5),
			Requirements: []string{"Go", "PostgreSQL"}, PostedAt: timePtr(now.Add(-time.Hour)),
		},
		{
			Source: "glints", SourceURL: "https://example.test/two", DedupKey: "feed-two",
			Title: "Go Platform Engineer", Company: "Beta", Location: "Jakarta Pusat",
			Remote: true, SalaryStatedMin: ptrI64(12_000_000), SalaryStatedMax: ptrI64(18_000_000),
			SalaryCurrency: "IDR", EmploymentType: "full_time", YearsExperience: ptrInt(2),
			Requirements: []string{"Go", "SQL"}, PostedAt: timePtr(now.Add(-2 * time.Hour)),
		},
		{
			Source: "glints", SourceURL: "https://example.test/three", DedupKey: "feed-three",
			Title: "Go Data Engineer", Company: "Gamma", Location: "Jakarta Barat",
			Remote: true, SalaryStatedMin: ptrI64(13_000_000), SalaryStatedMax: ptrI64(17_000_000),
			SalaryCurrency: "IDR", EmploymentType: "full_time",
			Requirements: []string{"Go", "SQL"}, PostedAt: timePtr(now.Add(-3 * time.Hour)),
		},
		{
			Source: "glints", SourceURL: "https://example.test/stale", DedupKey: "feed-stale",
			Title: "Stale Go Engineer", Company: "Delta", Location: "Jakarta Selatan",
			Remote: true, SalaryStatedMin: ptrI64(12_000_000), SalaryStatedMax: ptrI64(18_000_000),
			SalaryCurrency: "IDR", EmploymentType: "full_time", YearsExperience: ptrInt(2),
			Requirements: []string{"Go", "SQL"}, PostedAt: timePtr(now.Add(-4 * time.Hour)),
		},
	}
	for _, job := range jobs {
		if _, err := store.Upsert(ctx, job); err != nil {
			t.Fatalf("upsert %s: %v", job.DedupKey, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE jobs SET updated_at = now() - interval '72 hours' WHERE dedup_key = 'feed-stale'`); err != nil {
		t.Fatalf("age stale job: %v", err)
	}
	if _, err := store.SweepStale(ctx, 48*time.Hour); err != nil {
		t.Fatalf("sweep stale jobs: %v", err)
	}

	postedAfter := now.Add(-24 * time.Hour)
	remote := true
	maxYoE := 3
	q := Query{
		Search: "go", PayMin: ptrI64(11_000_000), PayMax: ptrI64(19_000_000),
		Location: "jakarta", Remote: &remote, Skills: []string{"Go", "SQL"},
		MaxYoE: &maxYoE, EmploymentType: "full_time", Source: "glints",
		PostedAfter: &postedAfter, Limit: 1,
	}
	page, err := store.SearchFeed(ctx, q)
	if err != nil {
		t.Fatalf("search feed: %v", err)
	}
	if page.Total != 2 || len(page.Jobs) != 1 || page.Jobs[0].DedupKey != "feed-two" {
		t.Fatalf("page 1 = total %d jobs %#v, want total 2 and feed-two", page.Total, page.Jobs)
	}
	q.Offset = 1
	page, err = store.SearchFeed(ctx, q)
	if err != nil {
		t.Fatalf("search feed page 2: %v", err)
	}
	if page.Total != 2 || len(page.Jobs) != 1 || page.Jobs[0].DedupKey != "feed-three" {
		t.Fatalf("page 2 = total %d jobs %#v, want total 2 and feed-three", page.Total, page.Jobs)
	}

	values := url.Values{}
	values.Set("q", "go")
	values.Set("pay_min", "11000000")
	values.Set("pay_max", "19000000")
	values.Set("location", "jakarta")
	values.Set("remote", "true")
	values.Set("skills", "Go,SQL")
	values.Set("max_yoe", "3")
	values.Set("employment_type", "full_time")
	values.Set("source", "glints")
	values.Set("posted_after", postedAfter.Format(time.RFC3339))
	values.Set("limit", "1")
	values.Set("offset", "0")
	rr := httptest.NewRecorder()
	NewService(store).Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed?"+values.Encode(), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("feed HTTP status = %d: %s", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); strings.Contains(strings.ToLower(body), "estimat") {
		t.Fatalf("feed response leaked estimated pay: %s", body)
	}
	var response feedResp
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode feed response: %v", err)
	}
	if response.Total != 2 || response.Limit != 1 || response.Offset != 0 || len(response.Jobs) != 1 {
		t.Fatalf("HTTP page = %#v, want total 2, limit 1, offset 0, one job", response)
	}
}

func TestPgxAndMemoryPaginationHaveStableTieBreakParity(t *testing.T) {
	store, _ := newFeedPgxStore(t)
	posted := time.Now().UTC().Truncate(time.Second)
	jobs := []ingest.Job{
		{
			Source: "fixture", SourceURL: "https://example.test/z", DedupKey: "z",
			Title: "Same Role", Company: "Acme", PostedAt: timePtr(posted),
		},
		{
			Source: "fixture", SourceURL: "https://example.test/a", DedupKey: "a",
			Title: "Same Role", Company: "Acme", PostedAt: timePtr(posted),
		},
	}
	for _, job := range jobs {
		if _, err := store.Upsert(context.Background(), job); err != nil {
			t.Fatalf("upsert %s: %v", job.DedupKey, err)
		}
	}

	for offset := range jobs {
		q := Query{Limit: 1, Offset: offset}
		memory := NewIndex(jobs).Search(q)
		postgres, err := store.SearchFeed(context.Background(), q)
		if err != nil {
			t.Fatalf("postgres offset %d: %v", offset, err)
		}
		if len(memory.Jobs) != 1 || len(postgres.Jobs) != 1 ||
			memory.Jobs[0].DedupKey != postgres.Jobs[0].DedupKey {
			t.Fatalf("offset %d memory=%#v postgres=%#v", offset, memory.Jobs, postgres.Jobs)
		}
	}
}

func ptrI64(v int64) *int64 { return &v }
func ptrInt(v int) *int     { return &v }
func timePtr(v time.Time) *time.Time {
	return &v
}
