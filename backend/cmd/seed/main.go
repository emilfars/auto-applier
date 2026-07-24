// Command seed loads synthetic Jabodetabek job listings into a job store so the
// feed is populated before signup opens (plan: seed >= 5,000 listings to avoid
// empty-feed churn). It is deterministic given -seed.
//
// At MVP this seeds an in-memory store and reports the count, exercising the
// shared seed.Seed routine that targets any ingest.JobStore. When the
// pgx-backed JobStore lands, point this command at it (via DATABASE_URL) to
// persist the same listings — no changes to the generator are required.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/seed"
)

func main() {
	n := flag.Int("n", seed.DefaultCount, "number of listings to seed")
	seedVal := flag.Int64("seed", 1, "PRNG seed for deterministic output")
	flag.Parse()

	store := ingest.NewMemoryStore()
	created, err := seed.Seed(context.Background(), store, *n, *seedVal, time.Now())
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seeded %d listings (requested %d)", created, *n)
	if created < seed.DefaultCount {
		log.Printf("warning: seeded fewer than the recommended %d listings", seed.DefaultCount)
	}
}
