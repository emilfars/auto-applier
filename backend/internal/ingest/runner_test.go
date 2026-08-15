package ingest

import (
	"context"
	"errors"
	"fmt"
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
	for _, job := range store.Active() {
		if job.Synthetic {
			t.Fatalf("registered real source job %q was marked synthetic", job.DedupKey)
		}
	}

	// Re-running the same listings updates rather than duplicates (dedup key).
	rep2 := r.RunOnce(context.Background())
	if rep2.TotalCreated() != 0 || store.Len() != 2 {
		t.Fatalf("second run created=%d store=%d, want 0/2", rep2.TotalCreated(), store.Len())
	}
}

func TestRunOnceEnforcesJabodetabekBoundary(t *testing.T) {
	cases := []struct {
		name     string
		location string
		remote   bool
		keep     bool
	}{
		{name: "Jakarta", location: "Jakarta Selatan", keep: true},
		{name: "Bogor", location: "Bogor, Jawa Barat", keep: true},
		{name: "Depok", location: "Depok, Jawa Barat", keep: true},
		{name: "Tangerang", location: "Tangerang Selatan, Banten", keep: true},
		{name: "Bekasi", location: "Bekasi, Jawa Barat", keep: true},
		{name: "Remote", location: "Bandung, Jawa Barat", remote: true, keep: true},
		{name: "Bandung", location: "Bandung, Jawa Barat"},
		{name: "Missing location", location: ""},
	}
	raws := make([]RawJob, 0, len(cases))
	for i, tc := range cases {
		raws = append(raws, RawJob{
			Source:    "boundary-board",
			SourceURL: fmt.Sprintf("https://boundary-board.example/jobs/%d", i),
			Title:     tc.name + " Role",
			Company:   "Boundary Co",
			Location:  tc.location,
			Remote:    tc.remote,
		})
	}

	reg := NewRegistry()
	if err := reg.Register(&fetchSource{id: "boundary-board", tier: Tier2, jobs: raws}); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := NewMemoryStore()
	report := NewRunner(reg, store, nil, 0, 0).RunOnce(context.Background())
	if report.TotalCreated() != 6 {
		t.Fatalf("created=%d, want 6 Jabodetabek/remote jobs", report.TotalCreated())
	}
	for _, tc := range cases {
		_, found := func() (Job, bool) {
			for _, job := range store.Active() {
				if job.Title == tc.name+" Role" {
					return job, true
				}
			}
			return Job{}, false
		}()
		if found != tc.keep {
			t.Errorf("%s location %q remote=%v found=%v, want %v", tc.name, tc.location, tc.remote, found, tc.keep)
		}
	}
}

// AC-SEED-1: a Postgres-backed fixture source reaches the launch threshold with
// real listings, while persisted synthetic and stale rows do not contribute.
func TestACSeed1RealFixtureListingsReachLaunchThreshold(t *testing.T) {
	store := newPgxTestStore(t)
	ctx := context.Background()

	synthetic := sampleJob("synthetic-existing", "Synthetic Existing Role")
	synthetic.Synthetic = true
	if _, err := store.Upsert(ctx, synthetic); err != nil {
		t.Fatalf("upsert synthetic row: %v", err)
	}
	stale := sampleJob("stale-existing", "Stale Existing Role")
	if _, err := store.Upsert(ctx, stale); err != nil {
		t.Fatalf("upsert stale row: %v", err)
	}
	if _, err := store.db.Exec(ctx,
		`UPDATE jobs SET updated_at = now() - interval '72 hours' WHERE dedup_key = 'stale-existing'`,
	); err != nil {
		t.Fatalf("age stale row: %v", err)
	}
	if _, err := store.SweepStale(ctx, StaleAfter); err != nil {
		t.Fatalf("sweep stale row: %v", err)
	}

	const fixtureCount = 5000
	posted := time.Now().UTC().Add(-time.Hour)
	fixtureJobs := make([]RawJob, fixtureCount)
	for i := range fixtureJobs {
		fixtureJobs[i] = RawJob{
			Source:         "fixture-tier1",
			SourceURL:      fmt.Sprintf("https://fixture.invalid/jobs/%d", i),
			Title:          fmt.Sprintf("Fixture Engineer %d", i),
			Company:        fmt.Sprintf("Fixture Company %d", i),
			Location:       "Jakarta Selatan",
			SalaryText:     "Rp 10.000.000 - Rp 15.000.000",
			EmploymentType: "Full Time",
			Requirements:   []string{"Go", "PostgreSQL"},
			PostedAt:       &posted,
		}
	}
	reg := NewRegistry()
	if err := reg.Register(&fetchSource{id: "fixture-tier1", tier: Tier1, jobs: fixtureJobs}); err != nil {
		t.Fatalf("register fixture source: %v", err)
	}

	report := NewRunner(reg, store, nil, 0, 0).RunOnce(ctx)
	if len(report.Sources) != 1 {
		t.Fatalf("source reports = %d, want 1", len(report.Sources))
	}
	sourceReport := report.Sources[0]
	if sourceReport.Err != nil || sourceReport.PersistErr != nil {
		t.Fatalf("fixture source report = %+v", sourceReport)
	}
	if sourceReport.Fetched != fixtureCount || sourceReport.Created != fixtureCount {
		t.Fatalf("fixture source report = %+v, want %d fetched and created", sourceReport, fixtureCount)
	}

	count, err := store.RealActiveCount(ctx)
	if err != nil {
		t.Fatalf("real active count: %v", err)
	}
	if count != fixtureCount {
		t.Fatalf("real active count = %d, want %d", count, fixtureCount)
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

type failingJobStore struct {
	err error
}

func (s failingJobStore) Upsert(context.Context, Job) (bool, error) {
	return false, s.err
}

func TestRunOnceReportsPersistenceFailuresSeparately(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&fetchSource{id: "board-a", tier: Tier2, jobs: []RawJob{
		rawJob("board-a", "Backend Engineer", "PT Alpha"),
	}})
	storeErr := errors.New("database unavailable")

	rep := NewRunner(reg, failingJobStore{err: storeErr}, nil, 0, 0).RunOnce(context.Background())
	got := rep.Sources[0]
	if got.Err != nil {
		t.Fatalf("fetch error = %v, want nil", got.Err)
	}
	if !errors.Is(got.PersistErr, storeErr) {
		t.Fatalf("persistence error = %v, want %v", got.PersistErr, storeErr)
	}
}

type blockingSource struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (s *blockingSource) ID() string { return "blocking" }
func (s *blockingSource) Tier() Tier { return Tier2 }
func (s *blockingSource) Fetch(context.Context) ([]RawJob, error) {
	s.started <- struct{}{}
	<-s.release
	return nil, nil
}

func TestRunOnceSerializesConcurrentCalls(t *testing.T) {
	reg := NewRegistry()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	_ = reg.Register(&blockingSource{started: started, release: release})
	r := NewRunner(reg, NewMemoryStore(), nil, 0, 0)
	done := make(chan RunReport, 2)

	go func() { done <- r.RunOnce(context.Background()) }()
	<-started
	secondAttempted := make(chan struct{})
	go func() {
		close(secondAttempted)
		done <- r.RunOnce(context.Background())
	}()
	<-secondAttempted

	select {
	case <-started:
		close(release)
		t.Fatal("RunOnce calls overlapped")
	default:
	}

	close(release)
	<-done
	<-done
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
