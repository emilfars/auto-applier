package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureJoobleJSON = `{
  "totalCount": 2,
  "jobs": [
    {
      "title": "Frontend Engineer",
      "location": "Jakarta, Indonesia",
      "snippet": "Senior React + TypeScript.",
      "salary": "Rp 12.000.000 - Rp 18.000.000",
      "type": "Full-time",
      "link": "https://jooble.org/desc/111",
      "company": "PT Bukalapak",
      "updated": "2026-06-01T10:00:00.0000000"
    },
    {
      "title": "Sales Manager",
      "location": "Bandung, Indonesia",
      "salary": "$3000 - $4000",
      "type": "Full-time",
      "link": "https://jooble.org/desc/222",
      "company": "PT Global",
      "updated": "2026-06-02"
    },
    {
      "title": "",
      "link": "https://jooble.org/desc/333",
      "company": "Ghost"
    }
  ]
}`

// AC-SCR-1 (Tier 1, Jooble): integration against a local fixture server — the
// keyed source posts the query, maps listings, drops non-IDR salaries, and the
// runner persists only Jabodetabek or explicitly remote listings.
func TestJoobleSourceAgainstFixtureServer(t *testing.T) {
	var gotKey string
	var gotBody joobleRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = strings.TrimPrefix(r.URL.Path, "/")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureJoobleJSON))
	}))
	defer srv.Close()

	src := NewJoobleSourceWithURL("jooble", srv.URL, "test-key", srv.Client())
	if src.Tier() != Tier1 {
		t.Fatalf("jooble source must be Tier 1, got %d", src.Tier())
	}

	reg := NewRegistry()
	if err := reg.Register(src); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := NewMemoryStore()
	rep := NewRunner(reg, store, nil, 0, 0).RunOnce(context.Background())

	if rep.TotalCreated() != 1 {
		t.Fatalf("created=%d, want 1 (Bandung and empty-title rows skipped)", rep.TotalCreated())
	}
	if gotKey != "test-key" {
		t.Errorf("api key path = %q, want test-key", gotKey)
	}
	if gotBody.Location != "Indonesia" {
		t.Errorf("request location = %q, want Indonesia", gotBody.Location)
	}

	jobs := store.All()
	var fe, sales *Job
	for i := range jobs {
		switch jobs[i].Title {
		case "Frontend Engineer":
			fe = &jobs[i]
		case "Sales Manager":
			sales = &jobs[i]
		}
	}
	if fe == nil {
		t.Fatalf("expected Jakarta job, got %+v", jobs)
	}
	if fe.SalaryStatedMin == nil || *fe.SalaryStatedMin != 12000000 ||
		fe.SalaryStatedMax == nil || *fe.SalaryStatedMax != 18000000 {
		t.Errorf("frontend salary = (%v,%v), want 12000000..18000000",
			fe.SalaryStatedMin, fe.SalaryStatedMax)
	}
	if fe.Seniority != "senior" {
		t.Errorf("frontend seniority = %q, want senior from snippet", fe.Seniority)
	}
	if len(fe.Requirements) != 1 || fe.Requirements[0] != "Senior React + TypeScript." {
		t.Errorf("frontend requirements = %#v, want Jooble snippet", fe.Requirements)
	}
	// USD salary must be dropped, not misparsed as IDR.
	if salary := joobleSalaryText("$3000 - $4000"); salary != "" {
		t.Errorf("non-IDR salary leaked: %q", salary)
	}
	if sales != nil {
		t.Fatalf("non-remote Bandung listing should be excluded: %+v", sales)
	}
}

// A source with no API key must fail loudly rather than silently returning
// nothing, so a misconfiguration is visible.
func TestJoobleSourceRequiresKey(t *testing.T) {
	src := NewJoobleSource("", nil)
	if _, err := src.Fetch(context.Background()); err == nil {
		t.Fatal("expected error when API key is empty")
	}
}

type joobleRoundTripper func(*http.Request) (*http.Response, error)

func (f joobleRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestJoobleSourceRedactsKeyFromTransportError(t *testing.T) {
	const secret = "jooble-secret"
	client := &http.Client{
		Transport: joobleRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		}),
	}
	src := NewJoobleSourceWithURL("jooble", "https://jooble.org/api", secret, client)

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed API key: %q", err)
	}
	if !strings.Contains(err.Error(), "ingest: fetch jooble") {
		t.Fatalf("error = %q, want source context", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error = %q, want transport cause", err)
	}
}

func joobleFixtureJobs(start, count int) []joobleJob {
	jobs := make([]joobleJob, 0, count)
	for i := 0; i < count; i++ {
		jobs = append(jobs, joobleJob{
			Title:   fmt.Sprintf("Role %d", start+i),
			Link:    fmt.Sprintf("https://jooble.test/%d", start+i),
			Company: "Fixture Co",
		})
	}
	return jobs
}

func TestJoobleSourcePaginatesAndEnforcesCap(t *testing.T) {
	var pages []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req joobleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		pages = append(pages, req.Page)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(joobleResponse{
			TotalCount: 5,
			Jobs:       joobleFixtureJobs((req.Page-1)*2, 2),
		})
	}))
	defer srv.Close()

	src := NewJoobleSourceWithURLAndLimit("jooble", srv.URL, "test-key", 4, srv.Client())
	jobs, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != 4 {
		t.Fatalf("jobs = %d, want cap 4", len(jobs))
	}
	if len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Fatalf("pages = %v, want [1 2]", pages)
	}
}

func TestJoobleSourceStopsOnShortPage(t *testing.T) {
	var pages []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req joobleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		pages = append(pages, req.Page)
		pageJobs := 2
		if req.Page == 3 {
			pageJobs = 1
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(joobleResponse{
			TotalCount: 10,
			Jobs:       joobleFixtureJobs((req.Page-1)*2, pageJobs),
		})
	}))
	defer srv.Close()

	src := NewJoobleSourceWithURLAndLimit("jooble", srv.URL, "test-key", 10, srv.Client())
	jobs, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != 5 {
		t.Fatalf("jobs = %d, want 5 before short-page exhaustion", len(jobs))
	}
	if len(pages) != 3 || pages[0] != 1 || pages[1] != 2 || pages[2] != 3 {
		t.Fatalf("pages = %v, want [1 2 3]", pages)
	}
}
