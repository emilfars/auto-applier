package ingest

import (
	"context"
	"sync"
	"time"
)

// JobStore persists normalized jobs. Upsert returns whether the job was newly
// created (vs. an update of an existing dedup_key).
type JobStore interface {
	Upsert(ctx context.Context, j Job) (created bool, err error)
}

// breaker is a per-source circuit breaker. After maxFailures consecutive
// failures it opens and short-circuits Fetch calls for cooldown, so a broken
// source cannot repeatedly stall a run. A single success closes it again.
type breaker struct {
	maxFailures int
	cooldown    time.Duration

	failures int
	openedAt time.Time
}

func (b *breaker) allow(now time.Time) bool {
	if b.failures < b.maxFailures {
		return true
	}
	// Open: stay short-circuited until the cooldown elapses (then half-open).
	return now.Sub(b.openedAt) >= b.cooldown
}

func (b *breaker) onSuccess() { b.failures = 0; b.openedAt = time.Time{} }

func (b *breaker) onFailure(now time.Time) {
	b.failures++
	if b.failures >= b.maxFailures {
		b.openedAt = now
	}
}

// SourceReport is the per-source outcome of a run.
type SourceReport struct {
	Source     string
	Fetched    int
	Created    int
	Updated    int
	Skipped    bool  // circuit breaker open
	Err        error // fetch error (isolated — does not fail the run)
	PersistErr error // normalized job persistence error
}

// RunReport aggregates a single ingestion pass.
type RunReport struct {
	Sources []SourceReport
}

// TotalCreated returns how many new jobs were persisted across all sources.
func (r RunReport) TotalCreated() int {
	n := 0
	for _, s := range r.Sources {
		n += s.Created
	}
	return n
}

// Runner ingests from all registered sources with per-source circuit breakers.
// A failing or open source is isolated: the run continues and healthy sources
// still populate the feed (SCR-1b).
type Runner struct {
	registry *Registry
	store    JobStore
	now      func() time.Time

	mu       sync.Mutex
	runMu    sync.Mutex // ponytail: global lock caps throughput at one pass; use per-source locks if throughput matters.
	breakers map[string]*breaker

	maxFailures int
	cooldown    time.Duration
}

// NewRunner builds a Runner. maxFailures/cooldown configure the per-source
// circuit breaker; zero values fall back to sensible defaults.
func NewRunner(reg *Registry, store JobStore, now func() time.Time, maxFailures int, cooldown time.Duration) *Runner {
	if now == nil {
		now = time.Now
	}
	if maxFailures <= 0 {
		maxFailures = 3
	}
	if cooldown <= 0 {
		cooldown = 10 * time.Minute
	}
	return &Runner{
		registry:    reg,
		store:       store,
		now:         now,
		breakers:    map[string]*breaker{},
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

func (r *Runner) breakerFor(id string) *breaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[id]
	if !ok {
		b = &breaker{maxFailures: r.maxFailures, cooldown: r.cooldown}
		r.breakers[id] = b
	}
	return b
}

// RunOnce performs one ingestion pass over all sources and returns a report.
// It never returns an error for an individual source failure — those are
// captured per-source so one bad source cannot abort the run (SCR-1b).
func (r *Runner) RunOnce(ctx context.Context) RunReport {
	r.runMu.Lock()
	defer r.runMu.Unlock()

	var report RunReport
	now := r.now()

	for _, src := range r.registry.Sources() {
		b := r.breakerFor(src.ID())
		sr := SourceReport{Source: src.ID()}

		if !b.allow(now) {
			sr.Skipped = true
			report.Sources = append(report.Sources, sr)
			continue
		}

		raws, err := src.Fetch(ctx)
		if err != nil {
			b.onFailure(now)
			sr.Err = err
			report.Sources = append(report.Sources, sr)
			continue
		}
		b.onSuccess()

		for _, raw := range raws {
			job, nerr := Normalize(raw)
			if nerr != nil {
				continue // drop malformed listings, keep going
			}
			if !isJabodetabekOrRemote(job) {
				continue
			}
			sr.Fetched++
			created, serr := r.store.Upsert(ctx, job)
			if serr != nil {
				sr.PersistErr = serr
				continue
			}
			if created {
				sr.Created++
			} else {
				sr.Updated++
			}
		}
		report.Sources = append(report.Sources, sr)
	}
	return report
}
