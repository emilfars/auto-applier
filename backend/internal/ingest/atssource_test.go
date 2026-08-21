package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	fixtureGreenhouseJSON = `{
  "jobs": [{
    "title": "Backend Engineer",
    "company_name": "PT Greenhouse",
    "location": {"name": "Jakarta Selatan"},
    "absolute_url": "https://boards.example.test/greenhouse/1",
    "first_published": "2026-06-01T10:00:00Z",
    "content": "<p>Go and PostgreSQL.</p>"
  }]
}`
	fixtureLeverJSON = `[{"text":"Platform Engineer","categories":{"location":"Remote","commitment":"Full-time"},"workplaceType":"remote","createdAt":1780308000000,"hostedUrl":"https://jobs.example.test/lever/1","descriptionPlain":"Build reliable systems."}]`
	fixtureWorkableJSON = `{
  "name": "PT Workable",
  "jobs": [{
    "title": "Product Analyst",
    "application_url": "https://apply.example.test/workable/1",
    "location": {"location_str": "Jakarta", "telecommuting": false},
    "employment_type": "Full-time",
    "salary": {"salary_from": 10000000, "salary_to": 15000000, "salary_currency": "IDR"},
    "published_on": "2026-06-01",
    "description": "<p>Analyze product data.</p>"
  }]
}`
	fixtureAshbyJSON = `{
  "jobs": [{
    "title": "Security Engineer",
    "location": "Singapore",
    "isListed": true,
    "isRemote": true,
    "workplaceType": "Remote",
    "jobUrl": "https://jobs.example.test/ashby/1",
    "employmentType": "FullTime",
    "publishedAt": "2026-06-01T10:00:00Z",
    "descriptionPlain": "Secure cloud infrastructure."
  }]
}`
)

// AC-SCR-8: each public ATS response shape normalizes through one source
// contract, including the bare-array Lever response and Workable's widget data.
func TestPublicATSSourcesAgainstFixtureShapes(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		build   func(string, string, *http.Client) *ATSSource
		wantLoc string
	}{
		{name: "greenhouse", payload: fixtureGreenhouseJSON, build: NewGreenhouseSourceWithURL, wantLoc: "Jakarta Selatan"},
		{name: "lever", payload: fixtureLeverJSON, build: NewLeverSourceWithURL, wantLoc: "Remote"},
		{name: "workable", payload: fixtureWorkableJSON, build: NewWorkableSourceWithURL, wantLoc: "Jakarta"},
		{name: "ashby", payload: fixtureAshbyJSON, build: NewAshbySourceWithURL, wantLoc: "Remote"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.payload))
			}))
			t.Cleanup(srv.Close)

			source := tc.build("fixture-company", srv.URL, srv.Client())
			if source.Tier() != Tier2 {
				t.Fatalf("tier = %d, want Tier 2", source.Tier())
			}
			store := NewMemoryStore()
			registry := NewRegistry()
			if err := registry.Register(source); err != nil {
				t.Fatalf("register: %v", err)
			}
			report := NewRunner(registry, store, nil, 0, 0).RunOnce(context.Background())
			if report.TotalCreated() != 1 {
				t.Fatalf("created=%d, want 1; report=%+v", report.TotalCreated(), report)
			}
			jobs := store.All()
			if len(jobs) != 1 || jobs[0].Location != tc.wantLoc {
				t.Fatalf("jobs=%+v, want one location %q", jobs, tc.wantLoc)
			}
			if len(jobs[0].Requirements) == 0 {
				t.Fatal("description was not retained as a requirement")
			}
		})
	}
}

// A dead slug is its own registered source. The healthy source still persists,
// and the dead one reports an isolated fetch error to the circuit breaker.
func TestATSSlugFailureIsolatedFromHealthySlug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dead" {
			http.Error(w, "missing board", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureGreenhouseJSON))
	}))
	t.Cleanup(srv.Close)

	registry := NewRegistry()
	if err := registry.Register(NewATSSourceWithURL(ATSGreenhouse, "dead", "Dead", srv.URL+"/dead", srv.Client())); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(NewATSSourceWithURL(ATSGreenhouse, "healthy", "Healthy", srv.URL+"/healthy", srv.Client())); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	report := NewRunner(registry, store, nil, 3, 0).RunOnce(context.Background())
	if report.TotalCreated() != 1 || store.Len() != 1 {
		t.Fatalf("healthy source did not persist: report=%+v store=%d", report, store.Len())
	}
	if report.Sources[0].Err == nil {
		t.Fatal("dead ATS slug should report a source error")
	}
	if report.Sources[1].Err != nil {
		t.Fatalf("healthy ATS slug failed: %v", report.Sources[1].Err)
	}
}

func TestATSCatalogIsDataDriven(t *testing.T) {
	for _, platform := range []ATSPlatform{ATSGreenhouse, ATSLever, ATSWorkable, ATSAshby} {
		entries := configuredATSCompanies(platform, nil)
		if len(entries) == 0 {
			t.Fatalf("%s catalog is empty", platform)
		}
		for _, entry := range entries {
			if entry.Slug == "" || entry.Company == "" {
				t.Fatalf("invalid %s catalog entry: %+v", platform, entry)
			}
		}
	}
}
