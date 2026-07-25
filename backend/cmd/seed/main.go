// Command seed populates a job store so the feed is not empty before signup.
//
// Two modes:
//
//	default (synthetic): loads deterministic Jabodetabek listings via the shared
//	  seed.Seed generator. Targets Postgres when DATABASE_URL is set, else an
//	  in-memory store. Used for the perf gate (large -n) and offline demos.
//
//	-real: pulls a small batch of REAL listings from the enabled live sources
//	  (Kalibrr Tier 2 always; Jooble Tier 1 when JOOBLE_API_KEY is set) into
//	  Postgres via the ingestion Runner. -n caps listings per source (25 for
//	  MVP). Requires DATABASE_URL.
//
// Neither mode fabricates salaries — listings carry employer-stated pay only.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/seed"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	n := flag.Int("n", seed.DefaultCount, "number of listings to seed (per source in -real mode)")
	seedVal := flag.Int64("seed", 1, "PRNG seed for deterministic synthetic output")
	real := flag.Bool("real", false, "pull real listings from live sources into Postgres")
	flag.Parse()

	if *real {
		seedReal(*n)
		return
	}

	// Synthetic mode: Postgres when configured, else in-memory.
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pool, closePool := openPool(url)
		defer closePool()
		seedInto(ingest.NewPgxStore(pool), *n, *seedVal)
		return
	}
	seedInto(ingest.NewMemoryStore(), *n, *seedVal)
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

// seedReal ingests a small real batch from the live sources into Postgres.
func seedReal(perSource int) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("-real requires DATABASE_URL")
	}
	if perSource <= 0 || perSource == seed.DefaultCount {
		perSource = 25 // MVP default: pull a small batch per board
	}

	pool, closePool := openPool(url)
	defer closePool()

	reg := ingest.BuildRegistry(ingest.SourceConfig{
		KalibrrLimit: perSource,
		JoobleAPIKey: os.Getenv("JOOBLE_API_KEY"),
	})
	if len(reg.Sources()) == 0 {
		log.Fatal("no live sources enabled (check network / JOOBLE_API_KEY)")
	}

	store := ingest.NewPgxStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rep := ingest.NewRunner(reg, store, time.Now, 0, 0).RunOnce(ctx)
	for _, s := range rep.Sources {
		switch {
		case s.Err != nil:
			log.Printf("source %-10s FAILED: %v", s.Source, s.Err)
		case s.Skipped:
			log.Printf("source %-10s skipped (circuit breaker open)", s.Source)
		default:
			log.Printf("source %-10s fetched=%d created=%d updated=%d",
				s.Source, s.Fetched, s.Created, s.Updated)
		}
	}
	log.Printf("real seed complete: %d new listing(s) persisted", rep.TotalCreated())
}

func openPool(url string) (*pgxpool.Pool, func()) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	applied, err := db.Apply(ctx, pool)
	if err != nil {
		pool.Close()
		log.Fatalf("apply migrations: %v", err)
	}
	log.Printf("database ready (%d migration(s) applied)", applied)
	return pool, pool.Close
}
