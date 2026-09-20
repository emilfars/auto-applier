package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const fixtureCareerjetJSON = `{
  "type": "JOBS",
  "hits": 2,
  "pages": 1,
  "jobs": [
    {
      "title": "Frontend Engineer",
      "company": "PT Careerjet",
      "locations": "Jakarta Selatan, Indonesia",
      "salary": "12,000,000 IDR - 18,000,000 IDR",
      "salary_currency_code": "IDR",
      "description": "<p>Senior React and TypeScript.</p>",
      "date": "Mon, 01 Jun 2026 10:00:00 GMT",
      "contract_type": "p",
      "url": "https://jobs.example.test/careerjet/frontend"
    },
    {
      "title": "Remote Analyst",
      "company": "PT Careerjet",
      "locations": "Remote",
      "salary": "USD 3000 - USD 4000",
      "salary_currency_code": "USD",
      "date": "Mon, 01 Jun 2026 11:00:00 GMT",
      "url": "https://jobs.example.test/careerjet/analyst"
    }
  ]
}`

// AC-SCR-7: keyed Careerjet fixture ingestion uses the Indonesian locale and
// Basic auth, while non-IDR salary is discarded rather than converted.
func TestCareerjetSourceAgainstFixtureServer(t *testing.T) {
	var gotQuery string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		user, password, ok := r.BasicAuth()
		if !ok || password != "" {
			t.Errorf("basic auth missing or password not empty")
		}
		gotAuth = user
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureCareerjetJSON))
	}))
	t.Cleanup(srv.Close)

	source := NewCareerjetSourceWithURLAndLimit("careerjet", srv.URL, "test-affid", 2, srv.Client())
	if source.Tier() != Tier1 {
		t.Fatalf("Careerjet tier = %d, want Tier 1", source.Tier())
	}
	store := NewMemoryStore()
	registry := NewRegistry()
	if err := registry.Register(source); err != nil {
		t.Fatalf("register: %v", err)
	}
	report := NewRunner(registry, store, nil, 0, 0).RunOnce(context.Background())
	if report.TotalCreated() != 2 {
		t.Fatalf("created=%d, want 2", report.TotalCreated())
	}
	if gotAuth != "test-affid" {
		t.Fatalf("basic auth user = %q, want test-affid", gotAuth)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"locale_code": "id_ID",
		"location":    "Indonesia",
		"page":        "1",
		"page_size":   "2",
		"user_ip":     "127.0.0.1",
	} {
		if values.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, values.Get(key), want)
		}
	}
	if values.Get("user_agent") == "" {
		t.Error("Careerjet user_agent query is empty")
	}
	if strings.Contains(gotQuery, "test-affid") {
		t.Error("affiliate id leaked into the URL query")
	}

	jobs := store.All()
	var frontend, analyst *Job
	for i := range jobs {
		switch jobs[i].Title {
		case "Frontend Engineer":
			frontend = &jobs[i]
		case "Remote Analyst":
			analyst = &jobs[i]
		}
	}
	if frontend == nil || analyst == nil {
		t.Fatalf("missing Careerjet jobs: %+v", jobs)
	}
	if frontend.SalaryStatedMin == nil || *frontend.SalaryStatedMin != 12_000_000 {
		t.Fatalf("frontend salary = %v, want 12000000", frontend.SalaryStatedMin)
	}
	if frontend.EmploymentType != "full_time" {
		t.Fatalf("frontend employment type = %q, want full_time", frontend.EmploymentType)
	}
	if analyst.SalaryStatedMin != nil || analyst.SalaryStatedMax != nil {
		t.Fatalf("foreign salary was carried as IDR: %+v", analyst)
	}
}

func TestCareerjetSourceRequiresAffiliateID(t *testing.T) {
	source := NewCareerjetSource("", nil)
	if _, err := source.Fetch(context.Background()); err == nil {
		t.Fatal("expected missing affiliate id error")
	}
}

// Fetch pages up to the run target instead of the old hard 10-page (1,000)
// cap; the server advertises many pages so only the target bounds the loop.
func TestCareerjetSourcePaginatesBeyondTenPages(t *testing.T) {
	const target = 1500
	var id int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if pageSize <= 0 {
			pageSize = 1
		}
		var b strings.Builder
		b.WriteString(`{"type":"JOBS","hits":100000,"pages":100000,"jobs":[`)
		for i := 0; i < pageSize; i++ {
			id++
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `{"title":"Role %d","company":"PT X","locations":"Jakarta, Indonesia","date":"Mon, 01 Jun 2026 10:00:00 GMT","url":"https://jobs.example.test/careerjet/%d"}`, id, id)
		}
		b.WriteString(`]}`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)

	source := NewCareerjetSourceWithURLAndLimit("careerjet", srv.URL, "test-affid", target, srv.Client())
	jobs, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jobs) != target {
		t.Fatalf("fetched %d jobs, want %d (old 10-page cap would stop at 1000)", len(jobs), target)
	}
}
