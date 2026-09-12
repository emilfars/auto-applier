package ingestctl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type stubCounter struct {
	count int
	err   error
}

func (s stubCounter) RealActiveCount(context.Context) (int, error) { return s.count, s.err }

func waitForRun(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ingestion run")
	}
}

func TestStartBackfillRunsWhenBelowTarget_AC(t *testing.T) {
	done := make(chan struct{})
	ctl := New(stubCounter{count: 10}, 5000, func(context.Context) { close(done) }, 0)

	started, err := ctl.StartBackfill(context.Background())
	if err != nil {
		t.Fatalf("StartBackfill() error = %v", err)
	}
	if !started {
		t.Fatal("StartBackfill() = false, want true when below target")
	}
	waitForRun(t, done)
}

func TestStartBackfillSkipsWhenSatisfied_AC(t *testing.T) {
	called := make(chan struct{}, 1)
	ctl := New(stubCounter{count: 5000}, 5000, func(context.Context) { called <- struct{}{} }, 0)

	started, err := ctl.StartBackfill(context.Background())
	if err != nil {
		t.Fatalf("StartBackfill() error = %v", err)
	}
	if started {
		t.Fatal("StartBackfill() = true, want false when at target")
	}
	select {
	case <-called:
		t.Fatal("run called for a satisfied feed")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartBackfillCountErrorDoesNotTrigger_AC(t *testing.T) {
	called := make(chan struct{}, 1)
	ctl := New(stubCounter{err: errors.New("database unavailable")}, 5000, func(context.Context) { called <- struct{}{} }, 0)

	started, err := ctl.StartBackfill(context.Background())
	if err == nil {
		t.Fatal("StartBackfill() error = nil, want count error")
	}
	if started {
		t.Fatal("StartBackfill() started despite count error")
	}
	select {
	case <-called:
		t.Fatal("run called after a count error")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartBackfillNilCounterIsNoop_AC(t *testing.T) {
	started, err := New(nil, 5000, nil, 0).StartBackfill(context.Background())
	if err != nil || started {
		t.Fatalf("StartBackfill() = (%v, %v), want (false, nil)", started, err)
	}
}

func TestTriggerIsSingleFlight_AC(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var runs int32
	ctl := New(nil, 5000, func(context.Context) {
		atomic.AddInt32(&runs, 1)
		close(started)
		<-release
	}, 0)

	if !ctl.Trigger() {
		t.Fatal("first Trigger() = false, want true")
	}
	waitForRun(t, started)

	if ctl.Trigger() {
		t.Fatal("second Trigger() = true while a run is in flight, want false")
	}
	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if ctl.Trigger() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Trigger() did not reopen after the run completed")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHandlerDisabledWithoutToken_AC(t *testing.T) {
	ctl := New(nil, 5000, nil, 0)
	rec := httptest.NewRecorder()
	ctl.Handler("").ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ingest/run", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandlerRejectsBadToken_AC(t *testing.T) {
	ctl := New(nil, 5000, nil, 0)
	req := httptest.NewRequest(http.MethodPost, "/ingest/run", nil)
	req.Header.Set("X-Ingest-Token", "wrong")
	rec := httptest.NewRecorder()
	ctl.Handler("secret").ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandlerStartsRunWithBearerToken_AC(t *testing.T) {
	done := make(chan struct{})
	ctl := New(nil, 5000, func(context.Context) { close(done) }, 0)

	req := httptest.NewRequest(http.MethodPost, "/ingest/run", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	ctl.Handler("secret").ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	waitForRun(t, done)
}

func TestHandlerConflictWhileRunning_AC(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	ctl := New(nil, 5000, func(context.Context) {
		close(started)
		<-release
	}, 0)

	if !ctl.Trigger() {
		t.Fatal("first Trigger() = false, want true")
	}
	waitForRun(t, started)

	req := httptest.NewRequest(http.MethodPost, "/ingest/run", nil)
	req.Header.Set("X-Ingest-Token", "secret")
	rec := httptest.NewRecorder()
	ctl.Handler("secret").ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	close(release)
}
