package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewCVStoreRequiresS3WithPersistentDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("CV_ENCRYPTION_KEY", strings.Repeat("00", 32))

	if _, err := newCVStore(context.Background()); err == nil {
		t.Fatal("expected persistent database configuration without S3 to fail")
	}
}

func TestNewCVStoreRequiresStableKeyWithS3(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("S3_ENDPOINT", "object-storage:9000")
	t.Setenv("CV_ENCRYPTION_KEY", "")

	_, err := newCVStore(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CV_ENCRYPTION_KEY") {
		t.Fatalf("expected missing persistent encryption key error, got %v", err)
	}
}

func TestIngestQueueConfigUsesStalenessDefaultAndOverride(t *testing.T) {
	t.Setenv("STALE_AFTER", "")
	got, err := ingestQueueConfig()
	if err != nil {
		t.Fatalf("default config: %v", err)
	}
	if got.StaleAfter != ingest.StaleAfter {
		t.Fatalf("StaleAfter default = %s, want %s", got.StaleAfter, ingest.StaleAfter)
	}

	for _, value := range []string{"48h", "1h"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("STALE_AFTER", value)
			got, err := ingestQueueConfig()
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			want, parseErr := time.ParseDuration(value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if got.StaleAfter != want {
				t.Fatalf("StaleAfter = %s, want %s", got.StaleAfter, want)
			}
		})
	}

	t.Setenv("STALE_AFTER", "72h")
	if _, err := ingestQueueConfig(); !errors.Is(err, queue.ErrStaleAfterTooLong) {
		t.Fatalf("expected threshold rejection, got %v", err)
	}
}

type testQueueClient struct {
	stopCalls int
}

func (c *testQueueClient) Stop(context.Context) error {
	c.stopCalls++
	return nil
}

func TestStartIngestQueueStartsWithNoSources(t *testing.T) {
	t.Setenv("KALIBRR_LIMIT", "0")
	t.Setenv("JOOBLE_LIMIT", "0")
	t.Setenv("JOOBLE_API_KEY", "")
	t.Setenv("ATS_PUBLIC_ENABLED", "false")
	t.Setenv("REMOTE_SOURCES_ENABLED", "false")
	t.Setenv("STALE_AFTER", "")

	var started bool
	client := &testQueueClient{}
	stop, _, err := startIngestQueueWith(
		&pgxpool.Pool{},
		ingest.NewPgxStore(&pgxpool.Pool{}),
		func(_ context.Context, _ *pgxpool.Pool, runner *ingest.Runner, _ *ingest.PgxStore, cfg queue.Config) (queueClient, error) {
			started = true
			if got := runner.RunOnce(context.Background()); len(got.Sources) != 0 {
				t.Fatalf("empty runner reported %d sources", len(got.Sources))
			}
			if cfg.StaleAfter != ingest.StaleAfter {
				t.Fatalf("stale threshold = %s, want %s", cfg.StaleAfter, ingest.StaleAfter)
			}
			if cfg.SweepInterval != time.Hour {
				t.Fatalf("sweep interval = %s, want 1h", cfg.SweepInterval)
			}
			return client, nil
		},
	)
	if err != nil {
		t.Fatalf("start queue: %v", err)
	}
	if !started {
		t.Fatal("expected queue startup attempt with no sources")
	}
	stop()
	if client.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", client.stopCalls)
	}
}

func TestStartIngestQueueNoPostgresIsNoop(t *testing.T) {
	stop, runner, err := startIngestQueue(nil, nil)
	if err != nil {
		t.Fatalf("start queue: %v", err)
	}
	if runner != nil {
		t.Fatal("expected no runner without Postgres")
	}
	stop()
}

func TestStartIngestQueuePropagatesStartError(t *testing.T) {
	wantErr := errors.New("river unavailable")
	_, _, err := startIngestQueueWith(
		&pgxpool.Pool{},
		ingest.NewPgxStore(&pgxpool.Pool{}),
		func(_ context.Context, _ *pgxpool.Pool, _ *ingest.Runner, _ *ingest.PgxStore, _ queue.Config) (queueClient, error) {
			return nil, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

type testJobCounter struct {
	count int
	err   error
	calls int
}

func (c *testJobCounter) RealActiveCount(context.Context) (int, error) {
	c.calls++
	return c.count, c.err
}

func TestRegistrationGate(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		err        error
		method     string
		path       string
		wantStatus int
		wantCalls  int
	}{
		{name: "below threshold blocks", count: 4999, wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "threshold permits", count: 5000, wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "count error fails closed", err: errors.New("database unavailable"), wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "login bypasses gate", err: errors.New("database unavailable"), method: http.MethodPost, path: "/auth/login", wantStatus: http.StatusNoContent},
		{name: "verify bypasses gate", err: errors.New("database unavailable"), method: http.MethodPost, path: "/auth/verify", wantStatus: http.StatusNoContent},
		{name: "reset bypasses gate", err: errors.New("database unavailable"), method: http.MethodPost, path: "/auth/password-reset/request", wantStatus: http.StatusNoContent},
		{name: "oauth bypasses gate", err: errors.New("database unavailable"), method: http.MethodPost, path: "/auth/oauth/google/callback", wantStatus: http.StatusNoContent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			counter := &testJobCounter{count: tc.count, err: tc.err}
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			handler := registrationGate(next, counter)
			method, path := tc.method, tc.path
			if method == "" {
				method, path = http.MethodPost, "/auth/register"
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if counter.calls != tc.wantCalls {
				t.Fatalf("count calls = %d, want %d", counter.calls, tc.wantCalls)
			}
			if tc.wantStatus == http.StatusServiceUnavailable &&
				!strings.Contains(rec.Body.String(), "5000 real active job listings") {
				t.Fatalf("service-unavailable body = %q, want clear seed message", rec.Body.String())
			}
		})
	}
}

func TestDemoJobStoreKeepsRegistrationClosed(t *testing.T) {
	t.Setenv("FEED_SEED_COUNT", "5000")
	store, counter, _ := newJobStore(nil)
	if store == nil {
		t.Fatal("demo store does not provide the feed")
	}
	if counter != nil {
		t.Fatal("demo store exposed a registration counter")
	}
	memory, ok := store.(*ingest.MemoryStore)
	if !ok {
		t.Fatalf("demo store type = %T, want *ingest.MemoryStore", store)
	}
	count, err := memory.RealActiveCount(context.Background())
	if err != nil {
		t.Fatalf("count demo jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("demo real active job count = %d, want 0", count)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	registrationGate(next, counter).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/auth/register", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("demo registration status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
