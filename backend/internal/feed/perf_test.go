package feed

import (
	"sort"
	"testing"
	"time"

	seedpkg "github.com/auto-applier/backend/internal/seed"
)

// AC-FEED-2b: filter query p95 < 500ms on a 50k-listing seed.
//
// This exercises the real endpoint path — rebuilding the Index from the active
// job snapshot and running the filters — across a spread of representative
// queries, then asserts the 95th-percentile latency stays under the budget.
func TestFilterP95Under500msOn50kSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 50k perf gate in -short mode")
	}

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jobs := seedpkg.NewGenerator(2026, base).Jobs(50000)
	if len(jobs) < 50000 {
		t.Fatalf("generated %d jobs, want 50000", len(jobs))
	}

	payMin := int64(8_000_000)
	payMax := int64(20_000_000)
	recent := base.Add(-14 * 24 * time.Hour)
	remote := true
	maxYoE := 3

	// A spread of filter combinations covering every FEED-2 predicate + search.
	queries := []Query{
		{},
		{Search: "engineer"},
		{Location: "Jakarta Selatan"},
		{Remote: &remote},
		{PayMin: &payMin, PayMax: &payMax},
		{Skills: []string{"Go", "SQL"}},
		{MaxYoE: &maxYoE},
		{EmploymentType: "full_time"},
		{Source: "glints"},
		{PostedAfter: &recent},
		{Search: "data", Location: "Jakarta", PayMin: &payMin, Skills: []string{"Python"}},
		{Search: "manager", MaxYoE: &maxYoE, EmploymentType: "contract", PostedAfter: &recent},
	}

	const iterations = 240
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		q := queries[i%len(queries)]
		start := time.Now()
		_ = NewIndex(jobs).Search(q)
		samples = append(samples, time.Since(start))
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[int(float64(len(samples))*0.95)]
	if p95 > 500*time.Millisecond {
		t.Fatalf("filter p95 = %v, want < 500ms on 50k seed", p95)
	}
	t.Logf("filter p95 on 50k seed: %v (budget 500ms)", p95)
}
