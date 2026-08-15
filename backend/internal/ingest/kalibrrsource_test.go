package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// A trimmed Kalibrr search response with the fields the source consumes,
// covering: an IDR stated salary, a non-IDR salary (must be dropped), a
// missing-salary listing, remote flag, and a title-less row (must be skipped).
const fixtureKalibrrJSON = `{
  "count": 4,
  "jobs": [
    {
      "id": 111,
      "name": "Backend Engineer",
      "slug": "backend-engineer",
      "company_name": "PT Tokopedia",
      "company": {"code": "pt-tokopedia", "name": "PT Tokopedia"},
      "tenure": "Full time",
      "salary_shown": true,
      "minimum_salary": 15000000,
      "maximum_salary": 25000000,
      "salary_currency": "IDR",
      "is_work_from_home": false,
      "description": "<p>Build APIs with <strong>Go</strong>.</p>",
      "qualifications": "<ul><li>PostgreSQL</li><li>Fresh graduates welcome</li></ul>",
      "is_open_to_fresh_grads": true,
      "activation_date": "2026-06-08T04:44:40.035983+00:00",
      "google_location": {"address_components": {"city": "Kota Jakarta Selatan", "region": "Daerah Khusus Ibukota Jakarta"}}
    },
    {
      "id": 222,
      "name": "Live Host",
      "slug": "live-host",
      "company_name": "PT Solutech",
      "company": {"code": "pt-solutech", "name": "PT Solutech"},
      "tenure": "Contractual",
      "salary_shown": true,
      "maximum_salary": 21322.5,
      "salary_currency": "PHP",
      "is_work_from_home": true,
      "google_location": {"address_components": {"city": "North Jakarta", "region": "Daerah Khusus Ibukota Jakarta"}}
    },
    {
      "id": 333,
      "name": "Data Analyst",
      "company_name": "PT Bandung",
      "company": {"code": "pt-bandung", "name": "PT Bandung"},
      "is_work_from_home": false,
      "google_location": {"address_components": {"city": "Bandung", "region": "Jawa Barat"}}
    },
    {
      "id": 0,
      "name": "  ",
      "company_name": "Ghost Co"
    }
  ]
}`

// AC-SCR-1 (Tier 2, Kalibrr): integration against a local fixture server — the
// JSON source fetches, maps, and the runner persists normalized Jabodetabek jobs;
// stated IDR salary parses, non-IDR salary is dropped, and non-local listings
// are excluded.
func TestKalibrrSourceAgainstFixtureServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("country"); got != "Indonesia" {
			t.Errorf("country query = %q, want Indonesia", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureKalibrrJSON))
	}))
	defer srv.Close()

	src := NewKalibrrSourceWithURL("kalibrr", srv.URL, 25, srv.Client())
	if src.Tier() != Tier2 {
		t.Fatalf("kalibrr source must be Tier 2, got %d", src.Tier())
	}

	reg := NewRegistry()
	if err := reg.Register(src); err != nil {
		t.Fatalf("register: %v", err)
	}
	store := NewMemoryStore()
	rep := NewRunner(reg, store, nil, 0, 0).RunOnce(context.Background())

	if rep.TotalCreated() != 2 {
		t.Fatalf("created=%d, want 2 (Bandung and title-less rows skipped)", rep.TotalCreated())
	}

	jobs := store.All()
	var backend, host *Job
	for i := range jobs {
		switch jobs[i].Title {
		case "Backend Engineer":
			backend = &jobs[i]
		case "Live Host":
			host = &jobs[i]
		}
	}

	if backend == nil || host == nil {
		t.Fatalf("expected Backend Engineer and Live Host, got %+v", jobs)
	}

	if backend.SalaryStatedMin == nil || *backend.SalaryStatedMin != 15000000 ||
		backend.SalaryStatedMax == nil || *backend.SalaryStatedMax != 25000000 {
		t.Errorf("backend salary = (%v,%v), want 15000000..25000000",
			backend.SalaryStatedMin, backend.SalaryStatedMax)
	}
	if backend.SourceURL != "https://www.kalibrr.com/c/pt-tokopedia/jobs/111/backend-engineer" {
		t.Errorf("backend source url = %q", backend.SourceURL)
	}
	if backend.Seniority != "entry" {
		t.Errorf("backend seniority = %q, want entry for fresh-grad role", backend.Seniority)
	}
	if len(backend.Requirements) != 2 ||
		!strings.Contains(backend.Requirements[0], "Build APIs with Go") ||
		!strings.Contains(backend.Requirements[1], "PostgreSQL") {
		t.Errorf("backend requirements = %#v, want description and qualifications text", backend.Requirements)
	}

	// PHP salary must not carry through — stated-IDR-only policy.
	if host.SalaryStatedMin != nil || host.SalaryStatedMax != nil {
		t.Errorf("non-IDR salary leaked: (%v,%v)", host.SalaryStatedMin, host.SalaryStatedMax)
	}
	if !host.Remote {
		t.Errorf("host should be remote (is_work_from_home)")
	}
	for _, job := range jobs {
		if job.Title == "Data Analyst" {
			t.Fatal("non-remote Bandung listing should be excluded")
		}
	}
}

func TestKalibrrSourcePaginatesAndEnforcesCap(t *testing.T) {
	const total = 130
	type request struct {
		limit  int
		offset int
	}
	var requests []request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		requests = append(requests, request{limit: limit, offset: offset})
		end := min(offset+limit, total)
		jobs := make([]kalibrrJob, 0, max(0, end-offset))
		for i := offset; i < end; i++ {
			jobs = append(jobs, kalibrrJob{ID: int64(i + 1), Name: fmt.Sprintf("Role %d", i+1)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kalibrrResponse{Count: total, Jobs: jobs})
	}))
	defer srv.Close()

	src := NewKalibrrSourceWithURL("kalibrr", srv.URL, 105, srv.Client())
	jobs, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != 105 {
		t.Fatalf("jobs = %d, want cap 105", len(jobs))
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if requests[0] != (request{limit: 100, offset: 0}) ||
		requests[1] != (request{limit: 5, offset: 100}) {
		t.Fatalf("requests = %#v, want bounded offset pages", requests)
	}
}

func TestKalibrrSourceStopsWhenCountIsExhausted(t *testing.T) {
	const total = 140
	var offsets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		offsets = append(offsets, offset)
		end := min(offset+limit, total)
		jobs := make([]kalibrrJob, 0, max(0, end-offset))
		for i := offset; i < end; i++ {
			jobs = append(jobs, kalibrrJob{ID: int64(i + 1), Name: fmt.Sprintf("Role %d", i+1)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kalibrrResponse{Count: total, Jobs: jobs})
	}))
	defer srv.Close()

	src := NewKalibrrSourceWithURL("kalibrr", srv.URL, 500, srv.Client())
	jobs, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != total {
		t.Fatalf("jobs = %d, want count %d", len(jobs), total)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 100 {
		t.Fatalf("offsets = %v, want [0 100]", offsets)
	}
}
