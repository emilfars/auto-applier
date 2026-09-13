package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AC-SCR-13: a cached validator produces a 304, and the source replays its last
// listing set so the runner still re-touches (upserts) live listings and the
// 48h staleness sweep cannot expire them.
func TestConditionalFetchReplaysCacheOn304(t *testing.T) {
	var requests int
	var sawValidator bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("If-None-Match") == `"v1"` {
			sawValidator = true
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureGreenhouseJSON))
	}))
	t.Cleanup(srv.Close)

	source := NewGreenhouseSourceWithURL("xendit", srv.URL, srv.Client())
	first, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first fetch jobs=%d, want 1", len(first))
	}

	second, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !sawValidator {
		t.Fatal("second request did not send If-None-Match")
	}
	if len(second) != 1 {
		t.Fatalf("304 replay jobs=%d, want 1 (cache must be re-touched)", len(second))
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want 2", requests)
	}
}

func TestConditionalFetchWithoutValidatorAlwaysFetches(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureGreenhouseJSON))
	}))
	t.Cleanup(srv.Close)

	source := NewGreenhouseSourceWithURL("xendit", srv.URL, srv.Client())
	for i := 0; i < 2; i++ {
		if _, err := source.Fetch(context.Background()); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want 2 unconditional fetches", requests)
	}
}
