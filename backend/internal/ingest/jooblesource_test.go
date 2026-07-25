package ingest

import (
	"context"
	"encoding/json"
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
      "snippet": "React + TypeScript.",
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
// runner persists them.
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

	if rep.TotalCreated() != 2 {
		t.Fatalf("created=%d, want 2 (empty-title row skipped)", rep.TotalCreated())
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
	if fe == nil || sales == nil {
		t.Fatalf("expected both jobs, got %+v", jobs)
	}
	if fe.SalaryStatedMin == nil || *fe.SalaryStatedMin != 12000000 ||
		fe.SalaryStatedMax == nil || *fe.SalaryStatedMax != 18000000 {
		t.Errorf("frontend salary = (%v,%v), want 12000000..18000000",
			fe.SalaryStatedMin, fe.SalaryStatedMax)
	}
	// USD salary must be dropped, not misparsed as IDR.
	if sales.SalaryStatedMin != nil || sales.SalaryStatedMax != nil {
		t.Errorf("non-IDR salary leaked: (%v,%v)", sales.SalaryStatedMin, sales.SalaryStatedMax)
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
