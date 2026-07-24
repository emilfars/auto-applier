package seed

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

var fixedBase = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

// AC-SEED-1: the seed job loads >= 5,000 distinct listings.
func TestSeedProducesAtLeast5000Distinct(t *testing.T) {
	store := ingest.NewMemoryStore()
	created, err := Seed(context.Background(), store, DefaultCount, 42, fixedBase)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if created < 5000 {
		t.Fatalf("created = %d, want >= 5000", created)
	}
	active, err := store.ActiveJobs(context.Background())
	if err != nil {
		t.Fatalf("ActiveJobs: %v", err)
	}
	if len(active) < 5000 {
		t.Fatalf("active = %d, want >= 5000 (dedup must not collapse distinct listings)", len(active))
	}
}

// Every generated listing must have a unique dedup key, otherwise the seed would
// silently collapse and under-fill the feed.
func TestGeneratedDedupKeysAreUnique(t *testing.T) {
	g := NewGenerator(7, fixedBase)
	seen := map[string]int{}
	const n = 6000
	for i := 0; i < n; i++ {
		j, err := ingest.Normalize(g.RawJob(i))
		if err != nil {
			t.Fatalf("Normalize(%d): %v", i, err)
		}
		if prev, ok := seen[j.DedupKey]; ok {
			t.Fatalf("dedup key collision between listing %d and %d", prev, i)
		}
		seen[j.DedupKey] = i
	}
	if len(seen) != n {
		t.Fatalf("unique keys = %d, want %d", len(seen), n)
	}
}

// The generator is Jabodetabek-first and stated-only: no fabricated estimates,
// and required identity fields are always present.
func TestGeneratedListingsAreValidAndJabodetabek(t *testing.T) {
	g := NewGenerator(1, fixedBase)
	jabo := []string{"Jakarta", "Bogor", "Depok", "Tangerang", "Bekasi"}
	for i := 0; i < 500; i++ {
		j, err := ingest.Normalize(g.RawJob(i))
		if err != nil {
			t.Fatalf("Normalize(%d): %v", i, err)
		}
		if j.SalaryCurrency != "IDR" {
			t.Fatalf("listing %d currency = %q, want IDR", i, j.SalaryCurrency)
		}
		ok := false
		for _, c := range jabo {
			if strings.Contains(j.Location, c) {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("listing %d location %q is not Jabodetabek", i, j.Location)
		}
		// Stated pay only: when a range is present, min <= max and both positive.
		if j.SalaryStatedMin != nil && j.SalaryStatedMax != nil {
			if *j.SalaryStatedMin > *j.SalaryStatedMax || *j.SalaryStatedMin <= 0 {
				t.Fatalf("listing %d bad salary range %d..%d", i, *j.SalaryStatedMin, *j.SalaryStatedMax)
			}
		}
	}
}

// Determinism: same seed => identical listings.
func TestGeneratorIsDeterministic(t *testing.T) {
	a := NewGenerator(99, fixedBase)
	b := NewGenerator(99, fixedBase)
	for i := 0; i < 200; i++ {
		ja, _ := ingest.Normalize(a.RawJob(i))
		jb, _ := ingest.Normalize(b.RawJob(i))
		if ja.DedupKey != jb.DedupKey || ja.Title != jb.Title || ja.Company != jb.Company {
			t.Fatalf("listing %d not deterministic", i)
		}
	}
}
