package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const fixtureSmartRecruitersJSON = `{
  "totalFound": 1,
  "content": [{
    "id": "744000012345",
    "name": "Merchant Relation Officer",
    "releasedDate": "2026-06-01T10:00:00.000Z",
    "company": {"name": "Cermati.com"},
    "location": {"city": "Jakarta", "region": "DKI Jakarta", "country": "id", "remote": false},
    "typeOfEmployment": {"label": "Full-time"},
    "ref": "https://api.smartrecruiters.com/v1/companies/Cermati/postings/744000012345"
  }]
}`

// AC-SCR-12: the SmartRecruiters source reads the paginated public postings API
// and normalizes location, employment type, and the public job URL.
func TestSmartRecruitersSourceFetch(t *testing.T) {
	var sawLimit, sawOffset bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sawLimit = q.Get("limit") != ""
		sawOffset = q.Get("offset") == "0"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSmartRecruitersJSON))
	}))
	t.Cleanup(srv.Close)

	company := SmartRecruitersCompany{Slug: "Cermati", Company: "Cermati.com"}
	source := NewSmartRecruitersSourceWithEndpoint(company, srv.URL, srv.Client())
	if source.Tier() != Tier2 {
		t.Fatalf("tier = %d, want Tier 2", source.Tier())
	}
	if source.ID() != "smartrecruiters-cermati" {
		t.Fatalf("id = %q", source.ID())
	}

	store := NewMemoryStore()
	registry := NewRegistry()
	if err := registry.Register(source); err != nil {
		t.Fatalf("register: %v", err)
	}
	report := NewRunner(registry, store, nil, 0, 0).RunOnce(context.Background())
	if !sawLimit || !sawOffset {
		t.Fatalf("pagination query missing: limit=%v offset=%v", sawLimit, sawOffset)
	}
	if report.TotalCreated() != 1 {
		t.Fatalf("created=%d, want 1; report=%+v", report.TotalCreated(), report)
	}
	jobs := store.All()
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d, want 1", len(jobs))
	}
	job := jobs[0]
	if job.Location != "Jakarta, DKI Jakarta, id" {
		t.Fatalf("location = %q", job.Location)
	}
	if job.EmploymentType != "full_time" {
		t.Fatalf("employment type = %q, want full_time", job.EmploymentType)
	}
	if job.SourceURL != "https://jobs.smartrecruiters.com/Cermati/744000012345" {
		t.Fatalf("source url = %q", job.SourceURL)
	}
}

func TestSmartRecruitersSourceRejectsMissingSlug(t *testing.T) {
	source := NewSmartRecruitersSourceWithEndpoint(SmartRecruitersCompany{}, "", nil)
	if _, err := source.Fetch(context.Background()); err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}
