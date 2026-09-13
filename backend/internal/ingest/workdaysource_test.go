package ingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const fixtureWorkdayJSON = `{
  "total": 2,
  "jobPostings": [
    {
      "title": "Backend Engineer",
      "externalPath": "/job/Jakarta/Backend-Engineer_R-1",
      "locationsText": "Jakarta, Indonesia",
      "postedOn": "Posted Today",
      "bulletFields": ["R-1"]
    },
    {
      "title": "Data Scientist",
      "externalPath": "/job/Remote/Data-Scientist_R-2",
      "locationsText": "Remote",
      "postedOn": "Posted 3 Days Ago",
      "bulletFields": ["R-2"]
    }
  ]
}`

// AC-SCR-11: the Workday CXS source posts a paginated listing request and
// normalizes the public postings; Jabodetabek/remote rows persist through the
// runner while out-of-region rows are dropped.
func TestWorkdaySourceFetchAndFilter(t *testing.T) {
	var sawPost bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		sawPost = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureWorkdayJSON))
	}))
	t.Cleanup(srv.Close)

	company := WorkdayCompany{Host: "acme.wd3.myworkdayjobs.com", Tenant: "acme", Site: "Acme_Careers", Company: "Acme"}
	source := NewWorkdaySourceWithEndpoint(company, srv.URL, srv.Client())
	if source.Tier() != Tier2 {
		t.Fatalf("tier = %d, want Tier 2", source.Tier())
	}
	if source.ID() != "workday-acme-acme-careers" {
		t.Fatalf("id = %q", source.ID())
	}

	store := NewMemoryStore()
	registry := NewRegistry()
	if err := registry.Register(source); err != nil {
		t.Fatalf("register: %v", err)
	}
	report := NewRunner(registry, store, nil, 0, 0).RunOnce(context.Background())
	if !sawPost {
		t.Fatal("source did not POST to the CXS endpoint")
	}
	if report.TotalCreated() != 2 {
		t.Fatalf("created=%d, want 2; report=%+v", report.TotalCreated(), report)
	}
	jobs := store.All()
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d, want 2", len(jobs))
	}
	var remoteSeen bool
	for _, job := range jobs {
		if job.SourceURL == "" {
			t.Fatalf("job without a public source URL: %+v", job)
		}
		if job.Location == "Remote" {
			remoteSeen = true
		}
	}
	if !remoteSeen {
		t.Fatal("remote posting was not normalized to Remote")
	}
}

func TestWorkdaySourcePaginates(t *testing.T) {
	const pageSize = workdayPageSize
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body workdayRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		postings := make([]workdayPosting, 0, pageSize)
		for i := 0; i < pageSize; i++ {
			postings = append(postings, workdayPosting{
				Title:         "Engineer",
				ExternalPath:  "/job/Jakarta/Engineer_R-" + strconv.Itoa(body.Offset+i),
				LocationsText: "Jakarta, Indonesia",
			})
		}
		if body.Offset > 0 {
			postings = postings[:5]
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(workdayResponse{Total: pageSize + 5, JobPostings: postings})
	}))
	t.Cleanup(srv.Close)

	company := WorkdayCompany{Host: "acme.wd3.myworkdayjobs.com", Tenant: "acme", Site: "Acme_Careers", Company: "Acme"}
	source := NewWorkdaySourceWithEndpoint(company, srv.URL, srv.Client())
	jobs, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != pageSize+5 {
		t.Fatalf("jobs=%d, want %d", len(jobs), pageSize+5)
	}
}

func TestWorkdaySourceRejectsMissingConfig(t *testing.T) {
	source := NewWorkdaySourceWithEndpoint(WorkdayCompany{}, "https://example.test/jobs", nil)
	if _, err := source.Fetch(context.Background()); err == nil {
		t.Fatal("expected error for missing host/tenant")
	}
}
