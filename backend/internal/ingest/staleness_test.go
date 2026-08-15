package ingest

import (
	"context"
	"testing"
	"time"
)

// AC-SCR-3: the same job posted on two different sources collapses to one
// canonical job in the store (dedup key).
func TestCrossSourceDeduplication(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&fetchSource{id: "jobstreet", tier: Tier2, jobs: []RawJob{
		{Source: "jobstreet", SourceURL: "https://jobstreet/x", Title: "Backend Engineer", Company: "PT Tokopedia", Location: "Jakarta Selatan, DKI Jakarta"},
	}})
	_ = reg.Register(&fetchSource{id: "glints", tier: Tier2, jobs: []RawJob{
		{Source: "glints", SourceURL: "https://glints/y", Title: "backend  engineer", Company: "pt tokopedia", Location: "Jakarta Selatan"},
	}})
	store := NewMemoryStore()

	NewRunner(reg, store, nil, 0, 0).RunOnce(context.Background())

	if store.Len() != 1 {
		t.Fatalf("cross-source duplicate not collapsed: store=%d, want 1", store.Len())
	}
}

func TestIsStaleRule(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		lastSeen time.Time
		now      time.Time
		want     bool
	}{
		{base, base.Add(47 * time.Hour), false},
		{base, base.Add(48 * time.Hour), true},
		{base, base.Add(72 * time.Hour), true},
		{time.Time{}, base, false}, // never-seen is not stale
	}
	for i, c := range cases {
		if got := IsStale(c.lastSeen, c.now, StaleAfter); got != c.want {
			t.Errorf("case %d: IsStale = %v, want %v", i, got, c.want)
		}
	}
}

// AC-SCR-4: a listing removed at the source (not re-seen) is marked stale within
// the 48h window; re-seeing it clears the flag.
func TestSweepStaleWithClockInjection(t *testing.T) {
	clock := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store := NewMemoryStoreClock(func() time.Time { return clock })

	job, _ := Normalize(RawJob{Source: "s", SourceURL: "https://example.test/u", Title: "Dev", Company: "PT Co", Location: "Jakarta"})
	if _, err := store.Upsert(context.Background(), job); err != nil {
		t.Fatal(err)
	}

	// Before 48h: still fresh.
	clock = clock.Add(47 * time.Hour)
	if n := store.SweepStale(StaleAfter); n != 0 {
		t.Fatalf("swept %d before window, want 0", n)
	}
	if store.IsStaleKey(job.DedupKey) {
		t.Fatal("job should not be stale before 48h")
	}

	// At/after 48h: marked stale and excluded from the active feed.
	clock = clock.Add(2 * time.Hour) // now 49h since last seen
	if n := store.SweepStale(StaleAfter); n != 1 {
		t.Fatalf("swept %d after window, want 1", n)
	}
	if !store.IsStaleKey(job.DedupKey) {
		t.Fatal("job should be stale after 48h")
	}
	if len(store.Active()) != 0 || len(store.All()) != 1 {
		t.Fatalf("active=%d all=%d, want 0/1", len(store.Active()), len(store.All()))
	}
	if count, err := store.RealActiveCount(context.Background()); err != nil || count != 0 {
		t.Fatalf("active count after sweep = %d, err=%v; want 0", count, err)
	}

	// Re-seeing the listing clears staleness.
	if _, err := store.Upsert(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if store.IsStaleKey(job.DedupKey) || len(store.Active()) != 1 {
		t.Fatal("re-seen job should be active again")
	}
	if count, err := store.RealActiveCount(context.Background()); err != nil || count != 1 {
		t.Fatalf("active count after re-seen = %d, err=%v; want 1", count, err)
	}
}
