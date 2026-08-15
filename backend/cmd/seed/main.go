// Command seed populates a job store so the feed is not empty before signup.
//
// Two modes:
//
//	default (synthetic): loads deterministic Jabodetabek listings via the shared
//	  seed.Seed generator into an in-memory store. Used for the perf gate (large
//	  -n) and offline demos. Refuses DATABASE_URL; use -real for Postgres.
//
//	-real: pulls the requested target of REAL listings from the enabled live sources
//	  (Kalibrr Tier 2 always; Jooble Tier 1 when JOOBLE_API_KEY is set) into
//	  Postgres via the ingestion Runner. -n is the total real-listing target,
//	  divided across enabled sources. Requires DATABASE_URL.
//
// Neither mode fabricates salaries — listings carry employer-stated pay only.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/seed"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	n := flag.Int("n", seed.DefaultCount, "number of listings to seed (total real-listing target in -real mode)")
	seedVal := flag.Int64("seed", 1, "PRNG seed for deterministic synthetic output")
	real := flag.Bool("real", false, "pull real listings from live sources into Postgres")
	flag.Parse()

	if *real {
		if err := seedReal(*n); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := validateSyntheticSeedTarget(os.Getenv("DATABASE_URL")); err != nil {
		log.Fatal(err)
	}
	seedInto(ingest.NewMemoryStore(), *n, *seedVal)
}

func validateSyntheticSeedTarget(databaseURL string) error {
	if databaseURL != "" {
		return errors.New("synthetic seeding refuses DATABASE_URL; use -real for persistent real listings")
	}
	return nil
}

func seedInto(store ingest.JobStore, n int, seedVal int64) {
	created, err := seed.Seed(context.Background(), store, n, seedVal, time.Now())
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seeded %d listings (requested %d)", created, n)
	if created < seed.DefaultCount {
		log.Printf("warning: seeded fewer than the recommended %d listings", seed.DefaultCount)
	}
}

// seedReal ingests the requested real-listing target from the live sources into Postgres.
func seedReal(target int) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("-real requires DATABASE_URL")
	}
	if target <= 0 {
		target = seed.DefaultCount
	}

	pool, closePool, err := openPool(url)
	if err != nil {
		return err
	}
	defer closePool()

	joobleKey := strings.TrimSpace(os.Getenv("JOOBLE_API_KEY"))
	kalibrrLimit, joobleLimit := realSeedCaps(target, joobleKey != "")
	if joobleLimit == 0 {
		joobleKey = ""
	}
	reg := ingest.BuildRegistry(ingest.SourceConfig{
		KalibrrLimit: kalibrrLimit,
		JoobleAPIKey: joobleKey,
		JoobleLimit:  joobleLimit,
	})
	if len(reg.Sources()) == 0 {
		return errors.New("no live sources enabled (check network / JOOBLE_API_KEY)")
	}

	store := ingest.NewPgxStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rep := ingest.NewRunner(reg, store, time.Now, 0, 0).RunOnce(ctx)
	for _, s := range rep.Sources {
		switch {
		case s.Err != nil:
			log.Printf("source %-10s FAILED: %v", s.Source, s.Err)
		case s.Skipped:
			log.Printf("source %-10s skipped (circuit breaker open)", s.Source)
		case s.PersistErr != nil:
			log.Printf("source %-10s PERSIST FAILED: %v (fetched=%d created=%d updated=%d)",
				s.Source, s.PersistErr, s.Fetched, s.Created, s.Updated)
		default:
			log.Printf("source %-10s fetched=%d created=%d updated=%d",
				s.Source, s.Fetched, s.Created, s.Updated)
		}
	}
	realCount, err := validateRealSeedResult(ctx, store, rep)
	if err != nil {
		return err
	}
	log.Printf("real seed complete: %d active real listing(s) available (%d new listing(s) persisted)",
		realCount, rep.TotalCreated())
	return nil
}

func realSeedCaps(target int, joobleEnabled bool) (kalibrr, jooble int) {
	if target <= 0 {
		target = seed.DefaultCount
	}
	kalibrr = target
	if joobleEnabled {
		jooble = target
	}
	return kalibrr, jooble
}

type realActiveCounter interface {
	RealActiveCount(context.Context) (int, error)
}

func validateRealSeedResult(ctx context.Context, store realActiveCounter, rep ingest.RunReport) (int, error) {
	realCount, countErr := store.RealActiveCount(ctx)
	var errs []error
	for _, source := range rep.Sources {
		if source.PersistErr != nil {
			errs = append(errs, fmt.Errorf("source %s persistence: %w", source.Source, source.PersistErr))
		}
	}
	if countErr != nil {
		errs = append(errs, fmt.Errorf("count active real listings: %w", countErr))
	} else if realCount < seed.DefaultCount {
		errs = append(errs, fmt.Errorf(
			"real seed has %d active real listing(s), need at least %d; more approved sources/listings are required",
			realCount, seed.DefaultCount))
	}
	if len(errs) > 0 {
		return realCount, fmt.Errorf("real seed failed: %w", errors.Join(errs...))
	}
	return realCount, nil
}

func openPool(url string) (*pgxpool.Pool, func(), error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, func() {}, fmt.Errorf("connect to database: %w", err)
	}
	applied, err := db.Apply(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, func() {}, fmt.Errorf("apply migrations: %w", err)
	}
	log.Printf("database ready (%d migration(s) applied)", applied)
	return pool, pool.Close, nil
}
