package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A trimmed Kalibrr search response with the fields the source consumes,
// covering: an IDR stated salary, a non-IDR salary (must be dropped), a
// missing-salary listing, remote flag, and a title-less row (must be skipped).
const fixtureKalibrrJSON = `{
  "count": 3,
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
      "id": 0,
      "name": "  ",
      "company_name": "Ghost Co"
    }
  ]
}`

// AC-SCR-1 (Tier 2, Kalibrr): integration against a local fixture server — the
// JSON source fetches, maps, and the runner persists normalized jobs; stated
// IDR salary parses, non-IDR salary is dropped, empty titles are skipped.
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
		t.Fatalf("created=%d, want 2 (title-less row skipped)", rep.TotalCreated())
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

	// PHP salary must not carry through — stated-IDR-only policy.
	if host.SalaryStatedMin != nil || host.SalaryStatedMax != nil {
		t.Errorf("non-IDR salary leaked: (%v,%v)", host.SalaryStatedMin, host.SalaryStatedMax)
	}
	if !host.Remote {
		t.Errorf("host should be remote (is_work_from_home)")
	}
}
