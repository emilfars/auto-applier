package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

func TestArgsKind(t *testing.T) {
	if (IngestArgs{}).Kind() != "ingest" {
		t.Errorf("IngestArgs.Kind() = %q", (IngestArgs{}).Kind())
	}
	if (SweepArgs{}).Kind() != "sweep_stale" {
		t.Errorf("SweepArgs.Kind() = %q", (SweepArgs{}).Kind())
	}
}

type queueTestSource struct {
	err error
}

func (s queueTestSource) ID() string        { return "board-a" }
func (s queueTestSource) Tier() ingest.Tier { return ingest.Tier2 }
func (s queueTestSource) Fetch(context.Context) ([]ingest.RawJob, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []ingest.RawJob{{
		Source:    "board-a",
		SourceURL: "https://board-a/jobs/1",
		Title:     "Backend Engineer",
		Company:   "PT Alpha",
		Location:  "Jakarta",
	}}, nil
}

type queueTestStore struct {
	err error
}

func (s queueTestStore) Upsert(context.Context, ingest.Job) (bool, error) {
	return false, s.err
}

func testQueueRunner(t *testing.T, source ingest.Source, store ingest.JobStore) *ingest.Runner {
	t.Helper()
	reg := ingest.NewRegistry()
	if err := reg.Register(source); err != nil {
		t.Fatal(err)
	}
	return ingest.NewRunner(reg, store, nil, 0, 0)
}

func TestIngestWorkerReturnsPersistenceError(t *testing.T) {
	storeErr := errors.New("database unavailable")
	w := ingestWorker{runner: testQueueRunner(t, queueTestSource{}, queueTestStore{err: storeErr})}

	err := w.Work(context.Background(), nil)
	if !errors.Is(err, storeErr) {
		t.Fatalf("error = %v, want wrapped %v", err, storeErr)
	}
}

func TestIngestWorkerIgnoresSourceFetchError(t *testing.T) {
	w := ingestWorker{runner: testQueueRunner(t, queueTestSource{err: errors.New("source unavailable")}, ingest.NewMemoryStore())}

	if err := w.Work(context.Background(), nil); err != nil {
		t.Fatalf("error = %v, want nil for isolated source failure", err)
	}
}

func TestConfigWithDefaults(t *testing.T) {
	got := Config{}.withDefaults()
	if got.IngestInterval != 6*time.Hour {
		t.Errorf("IngestInterval = %s, want 6h", got.IngestInterval)
	}
	if got.SweepInterval != time.Hour {
		t.Errorf("SweepInterval = %s, want 1h", got.SweepInterval)
	}
	if got.StaleAfter != ingest.StaleAfter {
		t.Errorf("StaleAfter = %s, want %s", got.StaleAfter, ingest.StaleAfter)
	}

	custom := Config{IngestInterval: time.Minute, SweepInterval: 2 * time.Minute, StaleAfter: time.Hour}
	if custom.withDefaults() != custom {
		t.Errorf("explicit values must be preserved, got %+v", custom.withDefaults())
	}
}

func TestConfigValidateStaleAfter(t *testing.T) {
	for _, tc := range []struct {
		name string
		age  time.Duration
		ok   bool
	}{
		{name: "smaller", age: time.Hour, ok: true},
		{name: "maximum", age: ingest.StaleAfter, ok: true},
		{name: "larger", age: ingest.StaleAfter + time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Config{StaleAfter: tc.age}).Validate()
			if tc.ok {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, ErrStaleAfterTooLong) {
				t.Fatalf("Validate() = %v, want %v", err, ErrStaleAfterTooLong)
			}
		})
	}
}
