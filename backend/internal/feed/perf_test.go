package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	seedpkg "github.com/auto-applier/backend/internal/seed"
	"github.com/jackc/pgx/v5"
)

// AC-FEED-2b: the real Postgres-backed HTTP feed stays below 500ms at p95.
func TestFeedHTTPP95Under500msOn50kPostgresRows(t *testing.T) {
	// AC-FEED-2b is an environment-sensitive perf assertion; per the plan's
	// "perf-stage, not per-commit" decision it runs only when RUN_PERF=1, so a
	// shared/underpowered CI runner does not fail the per-commit gate.
	if os.Getenv("RUN_PERF") != "1" {
		t.Skip("perf gate: set RUN_PERF=1 to run the 50k p95 check")
	}
	store, pool := newFeedPgxStore(t)
	ctx := context.Background()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jobs := seedpkg.NewGenerator(2026, base).Jobs(50_000)
	if len(jobs) != 50_000 {
		t.Fatalf("generated %d jobs, want 50000", len(jobs))
	}
	rows := make([][]any, len(jobs))
	for i, job := range jobs {
		requirements, err := json.Marshal(job.Requirements)
		if err != nil {
			t.Fatalf("marshal requirements: %v", err)
		}
		rows[i] = []any{
			job.Source, job.SourceURL, job.DedupKey, job.Title, job.Company, job.Location,
			job.Remote, job.SalaryStatedMin, job.SalaryStatedMax, job.SalaryCurrency,
			job.Seniority, job.EmploymentType, job.YearsExperience, requirements, job.PostedAt,
			job.Synthetic,
		}
	}
	if _, err := pool.CopyFrom(
		ctx,
		pgx.Identifier{"jobs"},
		[]string{
			"source", "source_url", "dedup_key", "title", "company", "location", "remote",
			"salary_stated_min", "salary_stated_max", "salary_currency", "seniority",
			"employment_type", "years_experience", "requirements", "posted_at", "synthetic",
		},
		pgx.CopyFromRows(rows),
	); err != nil {
		t.Fatalf("bulk seed: %v", err)
	}

	queries := perfQueries(base)
	server := httptest.NewServer(NewService(store).Routes())
	t.Cleanup(server.Close)
	transport := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     time.Minute,
	}
	client := &http.Client{Transport: transport}
	t.Cleanup(transport.CloseIdleConnections)
	for _, target := range queries {
		resp, err := client.Get(server.URL + target)
		if err != nil {
			t.Fatalf("warmup %s: %v", target, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("warmup %s: status %d", target, resp.StatusCode)
		}
	}

	const (
		requests = 240
		workers  = 32
	)
	samples := make([]time.Duration, requests)
	errs := make(chan error, requests)
	start := time.Now()
	var done sync.WaitGroup
	done.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(offset int) {
			defer done.Done()
			for i := offset; i < requests; i += workers {
				target := queries[i%len(queries)]
				requestStart := time.Now()
				resp, err := client.Get(server.URL + target)
				if err != nil {
					errs <- fmt.Errorf("%s: %w", target, err)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errs <- fmt.Errorf("%s: status %d", target, resp.StatusCode)
					continue
				}
				samples[i] = time.Since(requestStart)
			}
		}(worker)
	}
	done.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[len(samples)*95/100]
	t.Logf("AC-FEED-2b: %d concurrent HTTP samples in %v; p95=%v", requests, time.Since(start), p95)
	if p95 >= 500*time.Millisecond {
		t.Fatalf("feed HTTP/Postgres p95 = %v, want < 500ms", p95)
	}
}

func perfQueries(base time.Time) []string {
	payMin, payMax, maxYoE := "8000000", "20000000", "3"
	recent := base.Add(-14 * 24 * time.Hour).Format(time.RFC3339)
	values := []url.Values{
		{},
		{"q": {"engineer"}},
		{"location": {"Jakarta Selatan"}},
		{"remote": {"true"}},
		{"pay_min": {payMin}, "pay_max": {payMax}},
		{"skills": {"Go,SQL"}},
		{"max_yoe": {maxYoE}},
		{"employment_type": {"full_time"}},
		{"source": {"glints"}},
		{"posted_after": {recent}},
		{"q": {"data"}, "location": {"Jakarta"}, "pay_min": {payMin}, "skills": {"Python"}},
		{"q": {"manager"}, "max_yoe": {maxYoE}, "employment_type": {"contract"}, "posted_after": {recent}},
	}
	queries := make([]string, len(values))
	for i, value := range values {
		value.Set("limit", "20")
		queries[i] = "/feed?" + value.Encode()
	}
	return queries
}
