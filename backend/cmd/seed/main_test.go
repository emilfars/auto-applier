package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/seed"
)

func TestValidateSyntheticSeedTarget(t *testing.T) {
	t.Run("in-memory without database", func(t *testing.T) {
		if err := validateSyntheticSeedTarget(""); err != nil {
			t.Fatalf("validate synthetic target: %v", err)
		}
	})

	t.Run("rejects persistent database", func(t *testing.T) {
		err := validateSyntheticSeedTarget("postgres://example")
		if err == nil {
			t.Fatal("expected synthetic persistent seeding to fail")
		}
		if !strings.Contains(err.Error(), "-real") {
			t.Fatalf("error = %q, want -real guidance", err)
		}
	})
}

type fakeRealActiveCounter struct {
	count int
	err   error
}

func (f fakeRealActiveCounter) RealActiveCount(context.Context) (int, error) {
	return f.count, f.err
}

func TestValidateRealSeedResult(t *testing.T) {
	t.Run("persistence failure", func(t *testing.T) {
		want := errors.New("disk full")
		count, err := validateRealSeedResult(context.Background(), fakeRealActiveCounter{
			count: seed.DefaultCount,
		}, ingest.RunReport{Sources: []ingest.SourceReport{{
			Source:     "kalibrr",
			PersistErr: want,
		}}})
		if count != seed.DefaultCount {
			t.Fatalf("count = %d, want %d", count, seed.DefaultCount)
		}
		if err == nil || !errors.Is(err, want) {
			t.Fatalf("error = %v, want persistence error", err)
		}
	})

	t.Run("below real listing gate", func(t *testing.T) {
		count, err := validateRealSeedResult(context.Background(), fakeRealActiveCounter{count: 4999}, ingest.RunReport{})
		if count != 4999 {
			t.Fatalf("count = %d, want 4999", count)
		}
		if err == nil || !strings.Contains(err.Error(), "more approved sources/listings are required") {
			t.Fatalf("error = %v, want below-target guidance", err)
		}
	})
}

func TestRealSeedCaps(t *testing.T) {
	if kalibrr, jooble := realSeedCaps(seed.DefaultCount, false); kalibrr != seed.DefaultCount || jooble != 0 {
		t.Fatalf("without Jooble caps = %d/%d, want %d/0", kalibrr, jooble, seed.DefaultCount)
	}
	if kalibrr, jooble := realSeedCaps(seed.DefaultCount, true); kalibrr != seed.DefaultCount || jooble != seed.DefaultCount {
		t.Fatalf("with Jooble caps = %d/%d, want %d/%d", kalibrr, jooble, seed.DefaultCount, seed.DefaultCount)
	}
}
