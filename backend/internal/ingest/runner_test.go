package ingest

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fetchSource is a configurable Source stub.
type fetchSource struct {
	id   string
	tier Tier
	jobs []RawJob
	err  error
}

func (s *fetchSource) ID() string { return s.id }
func (s *fetchSource) Tier() Tier { return s.tier }
func (s *fetchSource) Fetch(context.Context) ([]RawJob, error) {
	return s.jobs, s.err
}

func rawJob(source, title, company string) RawJob {
	return RawJob{Source: source, SourceURL: "https://" + source + "/" + title, Title: title, Company: company, Location: "Jakarta"}
}

// AC-SCR-1: a run fetches from a source and persists normalized jobs.
func TestRunOncePersistsJobs(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&fetchSource{id: "board-a", tier: Tier2, jobs: []RawJob{
		rawJob("board-a", "Backend Engineer", "PT Alpha"),
		rawJob("board-a", "Frontend Engineer", "PT Alpha"),
	}})
	store := NewMemoryStore()
	r := NewRunner(reg, store, nil, 0, 0)

	rep := r.RunOnce(context.Background())
	if rep.TotalCreated() != 2 || store.Len() != 2 {
		t.Fatalf("created=%d store=%d, want 2/2", rep.TotalCreated(), store.Len())
	}

	// Re-running the same listings updates rather than duplicates (dedup key).
	rep2 := r.RunOnce(context.Background())
	if rep2.TotalCreated() != 0 || store.Len() != 2 {
		t.Fatalf("second run created=%d store=%d, want 0/2", rep2.TotalCreated(), store.Len())
	}
}

// AC-SCR-1b: one failing source does not fail the run or empty the feed.
func TestRunOnceIsolatesFailingSource(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&fetchSource{id: "broken", tier: Tier2, err: errors.New("boom")})
	_ = reg.Register(&fetchSource{id: "healthy", tier: Tier2, jobs: []RawJob{
		rawJob("healthy", "Data Analyst", "PT Beta"),
	}})
	store := NewMemoryStore()
	r := NewRunner(reg, store, nil, 3, time.Minute)

	rep := r.RunOnce(context.Background())

	if store.Len() != 1 {
		t.Fatalf("feed should be non-empty despite broken source: store=%d", store.Len())
	}
	var brokenReport, healthyReport SourceReport
	for _, s := range rep.Sources {
		switch s.Source {
		case "broken":
			brokenReport = s
		case "healthy":
			healthyReport = s
		}
	}
	if brokenReport.Err == nil {
		t.Error("broken source should record its error")
	}
	if healthyReport.Created != 1 {
		t.Errorf("healthy source created=%d, want 1", healthyReport.Created)
	}
}

// After maxFailures consecutive failures the breaker opens and the source is
// skipped until the cooldown elapses.
func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	reg := NewRegistry()
	src := &fetchSource{id: "flaky", tier: Tier2, err: errors.New("down")}
	_ = reg.Register(src)
	store := NewMemoryStore()

	clock := time.Unix(0, 0)
	r := NewRunner(reg, store, func() time.Time { return clock }, 2, 5*time.Minute)

	// Two failures open the breaker.
	r.RunOnce(context.Background())
	r.RunOnce(context.Background())

	// Third run: breaker open -> skipped, Fetch not attempted.
	rep := r.RunOnce(context.Background())
	if !rep.Sources[0].Skipped {
		t.Fatal("breaker should be open (source skipped)")
	}

	// After cooldown the source recovers and ingests.
	src.err = nil
	src.jobs = []RawJob{rawJob("flaky", "QA Engineer", "PT Gamma")}
	clock = clock.Add(6 * time.Minute)
	rep = r.RunOnce(context.Background())
	if rep.Sources[0].Skipped || rep.Sources[0].Created != 1 {
		t.Fatalf("source should recover after cooldown: %+v", rep.Sources[0])
	}
	if store.Len() != 1 {
		t.Fatalf("store=%d, want 1 after recovery", store.Len())
	}
}
