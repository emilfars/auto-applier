package seed

import (
	"context"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

// DefaultCount is the minimum listing volume the plan requires before signup
// opens, to avoid empty-feed churn.
const DefaultCount = 5000

// Seed generates n deterministic listings and upserts them into the store,
// returning the number of newly created listings. Because every generated
// listing has a unique (company, title, city) identity, a fresh store gains n
// distinct jobs (no dedup collapse). It works against any ingest.JobStore, so
// the same job seeds the in-memory store today and a pgx-backed store later.
func Seed(ctx context.Context, store ingest.JobStore, n int, seedVal int64, baseTime time.Time) (int, error) {
	g := NewGenerator(seedVal, baseTime)
	created := 0
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return created, err
		}
		j, err := ingest.Normalize(g.RawJob(i))
		if err != nil {
			continue
		}
		wasNew, err := store.Upsert(ctx, j)
		if err != nil {
			return created, err
		}
		if wasNew {
			created++
		}
	}
	return created, nil
}
