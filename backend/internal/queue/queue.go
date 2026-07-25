// Package queue wires the River job queue: a periodic ingestion worker that
// runs the source Runner, and a periodic staleness-sweep worker that expires
// listings no source has reconfirmed (SCR-4). River is used only when Postgres
// is configured; without a pool the API falls back to the boot-time synthetic
// seed and no background scheduling runs.
package queue

import (
	"context"
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
	for _, s := range rep.Sources {
		if s.Err != nil {
			log.Printf("river ingest: source %s failed: %v", s.Source, s.Err)
		}
	}
	log.Printf("river ingest: %d new listing(s) across %d source(s)", rep.TotalCreated(), len(rep.Sources))
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
}

func (c Config) withDefaults() Config {
	if c.IngestInterval <= 0 {
		c.IngestInterval = 6 * time.Hour
	}
	if c.SweepInterval <= 0 {
		c.SweepInterval = time.Hour
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = 14 * 24 * time.Hour
	}
	return c
}

// Start migrates River's schema, registers the workers, schedules the periodic
// ingestion and sweep jobs, and starts the client. The returned client should
// be Stopped on shutdown. Ingestion runs once on start so a fresh deployment
// populates the feed promptly.
func Start(ctx context.Context, pool *pgxpool.Pool, runner *ingest.Runner, store *ingest.PgxStore, cfg Config) (*river.Client[pgx.Tx], error) {
	cfg = cfg.withDefaults()
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
				&river.PeriodicJobOpts{RunOnStart: true},
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
