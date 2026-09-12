// Package queue wires the River job queue: a periodic ingestion worker that
// runs the source Runner, and a periodic staleness-sweep worker that expires
// listings no source has reconfirmed (SCR-4). River is used only when Postgres
// is configured; without a pool the API falls back to the boot-time synthetic
// seed and no background scheduling runs.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// IngestArgs triggers one ingestion pass over all registered sources.
type IngestArgs struct{}

// Kind is River's stable job-type identifier.
func (IngestArgs) Kind() string { return "ingest" }

// SweepArgs triggers one staleness sweep.
type SweepArgs struct{}

// Kind is River's stable job-type identifier.
func (SweepArgs) Kind() string { return "sweep_stale" }

type ingestWorker struct {
	river.WorkerDefaults[IngestArgs]
	runner *ingest.Runner
}

func (w *ingestWorker) Work(ctx context.Context, _ *river.Job[IngestArgs]) error {
	rep := w.runner.RunOnce(ctx)
	var persistErrs []error
	for _, s := range rep.Sources {
		if s.Err != nil {
			log.Printf("river ingest: source %s failed: %v", s.Source, s.Err)
		}
		if s.PersistErr != nil {
			persistErrs = append(persistErrs, fmt.Errorf("source %s: %w", s.Source, s.PersistErr))
		}
	}
	log.Printf("river ingest: %d new listing(s) across %d source(s)", rep.TotalCreated(), len(rep.Sources))
	if len(persistErrs) > 0 {
		return fmt.Errorf("queue ingest persistence: %w", errors.Join(persistErrs...))
	}
	return nil
}

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	store  *ingest.PgxStore
	maxAge time.Duration
}

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	n, err := w.store.SweepStale(ctx, w.maxAge)
	if err != nil {
		return fmt.Errorf("queue: sweep stale: %w", err)
	}
	if n > 0 {
		log.Printf("river sweep: expired %d stale listing(s)", n)
	}
	return nil
}

// Config controls the periodic schedules and staleness threshold.
type Config struct {
	IngestInterval time.Duration // how often to re-ingest sources
	SweepInterval  time.Duration // how often to sweep stale listings
	StaleAfter     time.Duration // age past which a listing is expired
	// RunOnStart fires one ingestion pass the moment the queue starts. It
	// defaults to false so a healthy deployment does not re-fetch every source
	// on every restart; the API instead backfills only when the active
	// real-listing count is low (internal/ingestctl).
	RunOnStart bool
}

// ErrStaleAfterTooLong identifies a queue threshold wider than the ingest rule.
var ErrStaleAfterTooLong = errors.New("queue staleness threshold exceeds ingest.StaleAfter")

func (c Config) withDefaults() Config {
	if c.IngestInterval <= 0 {
		c.IngestInterval = 6 * time.Hour
	}
	if c.SweepInterval <= 0 {
		c.SweepInterval = time.Hour
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = ingest.StaleAfter
	}
	return c
}

// Validate rejects a staleness threshold wider than the ingest rule.
func (c Config) Validate() error {
	if c.StaleAfter > ingest.StaleAfter {
		return fmt.Errorf("stale after %s exceeds maximum %s: %w", c.StaleAfter, ingest.StaleAfter, ErrStaleAfterTooLong)
	}
	return nil
}

// Start migrates River's schema, registers the workers, schedules the periodic
// ingestion and sweep jobs, and starts the client. The returned client should
// be Stopped on shutdown. The immediate ingest pass on start is opt-in via
// Config.RunOnStart; the API backfills at boot only when the feed is below the
// launch target.
func Start(ctx context.Context, pool *pgxpool.Pool, runner *ingest.Runner, store *ingest.PgxStore, cfg Config) (*river.Client[pgx.Tx], error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("queue: invalid configuration: %w", err)
	}
	driver := riverpgxv5.New(pool)

	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		return nil, fmt.Errorf("queue: new migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return nil, fmt.Errorf("queue: migrate river schema: %w", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &ingestWorker{runner: runner})
	river.AddWorker(workers, &sweepWorker{store: store, maxAge: cfg.StaleAfter})

	client, err := river.NewClient(driver, &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 2}},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(cfg.IngestInterval),
				func() (river.JobArgs, *river.InsertOpts) { return IngestArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: cfg.RunOnStart},
			),
			river.NewPeriodicJob(
				river.PeriodicInterval(cfg.SweepInterval),
				func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: false},
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("queue: new river client: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("queue: start river client: %w", err)
	}
	log.Printf("river started (ingest every %s, sweep every %s, stale after %s)",
		cfg.IngestInterval, cfg.SweepInterval, cfg.StaleAfter)
	return client, nil
}
