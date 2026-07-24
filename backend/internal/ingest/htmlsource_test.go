package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const fixtureListingHTML = `<!doctype html>
<html><body>
  <div class="job-card">
    <h2 class="job-title">Backend Engineer</h2>
    <span class="job-company">PT Tokopedia</span>
    <span class="job-loc">Jakarta Selatan, DKI Jakarta</span>
    <span class="job-salary">Rp 15.000.000 - Rp 25.000.000</span>
    <span class="job-type">Full Time</span>
    <a class="job-link" href="/jobs/backend-123">Detail</a>
  </div>
  <div class="job-card">
    <h2 class="job-title">Data Analyst</h2>
    <span class="job-company">PT Traveloka</span>
    <span class="job-loc">Remote</span>
    <span class="job-salary">8 - 12 juta</span>
    <span class="job-type">Kontrak</span>
    <a class="job-link" href="https://external.example.com/jobs/da-9">Detail</a>
  </div>
</body></html>`

// AC-SCR-1: integration against a local fixture HTML server — the HTML source
// fetches and the runner persists normalized jobs.
func TestHTMLSourceAgainstFixtureServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureListingHTML))
	}))
	defer srv.Close()

	sel := CardSelectors{
		Card: "job-card", Title: "job-title", Company: "job-company",
		Location: "job-loc", Salary: "job-salary", Employment: "job-type", URL: "job-link",
	}
	src := NewHTMLSource("fixture-board", srv.URL, sel, srv.Client())

	if src.Tier() != Tier2 {
		t.Fatalf("HTML source must be Tier 2, got %d", src.Tier())
	}

	reg := NewRegistry()
	if err := reg.Register(src); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := NewMemoryStore()
	rep := NewRunner(reg, store, nil, 0, 0).RunOnce(context.Background())

	if rep.TotalCreated() != 2 {
		t.Fatalf("created=%d, want 2", rep.TotalCreated())
	}

	jobs := store.All()
	var backend, analyst *Job
	for i := range jobs {
		switch jobs[i].Title {
		case "Backend Engineer":
			backend = &jobs[i]
		case "Data Analyst":
			analyst = &jobs[i]
		}
	}
	if backend == nil || analyst == nil {
		t.Fatalf("missing parsed jobs: %+v", jobs)
	}
	if backend.Company != "PT Tokopedia" || backend.EmploymentType != "full_time" {
		t.Errorf("backend normalized wrong: %+v", backend)
	}
	if backend.SalaryStatedMin == nil || *backend.SalaryStatedMin != 15_000_000 {
		t.Errorf("backend salary min = %v", backend.SalaryStatedMin)
	}
	// Relative href resolved against the page URL.
	if backend.SourceURL != srv.URL+"/jobs/backend-123" {
		t.Errorf("relative url not resolved: %q", backend.SourceURL)
	}
	// Absolute href preserved.
	if analyst.SourceURL != "https://external.example.com/jobs/da-9" {
		t.Errorf("absolute url = %q", analyst.SourceURL)
	}
	if !analyst.Remote {
		t.Error("analyst location 'Remote' should set remote=true")
	}
	if analyst.EmploymentType != "contract" {
		t.Errorf("analyst employment = %q, want contract", analyst.EmploymentType)
	}
}

func TestHTMLSourceNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	src := NewHTMLSource("x", srv.URL, CardSelectors{Card: "job-card"}, srv.Client())
	if _, err := src.Fetch(context.Background()); err == nil {
		t.Fatal("expected error on non-200")
	}
}
