package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

func i64(v int64) *int64          { return &v }
func iptr(v int) *int             { return &v }
func tptr(t time.Time) *time.Time { return &t }

var base = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

// seed returns a small, controlled dataset for filter-count assertions.
func seed() []ingest.Job {
	return []ingest.Job{
		{
			Source: "jobstreet", SourceURL: "https://a", Title: "Senior Backend Engineer", Company: "PT Tokopedia",
			Location: "Jakarta Selatan", Remote: false, SalaryStatedMin: i64(15_000_000), SalaryStatedMax: i64(25_000_000),
			SalaryCurrency: "IDR", EmploymentType: "full_time", YearsExperience: iptr(5),
			Requirements: []string{"Go", "PostgreSQL"}, PostedAt: tptr(base),
		},
		{
			Source: "glints", SourceURL: "https://b", Title: "Frontend Engineer", Company: "PT Bukalapak",
			Location: "Bandung", Remote: true, SalaryStatedMin: i64(8_000_000), SalaryStatedMax: i64(12_000_000),
			SalaryCurrency: "IDR", EmploymentType: "contract", YearsExperience: iptr(2),
			Requirements: []string{"React", "TypeScript"}, PostedAt: tptr(base.Add(24 * time.Hour)),
		},
		{
			Source: "kalibrr", SourceURL: "https://c", Title: "Data Analyst", Company: "PT Traveloka",
			Location: "Jakarta Pusat", Remote: true, SalaryCurrency: "IDR", EmploymentType: "full_time",
			Requirements: []string{"SQL", "Python"}, PostedAt: tptr(base.Add(48 * time.Hour)),
		},
	}
}

type stubProvider struct{ jobs []ingest.Job }

func (s stubProvider) ActiveJobs(context.Context) ([]ingest.Job, error) { return s.jobs, nil }

func countFor(t *testing.T, q Query) int {
	t.Helper()
	return NewIndex(seed()).Search(q).Total
}

// AC-FEED-2: each filter returns the correct subset.
func TestFilters(t *testing.T) {
	trueVal := true
	cases := []struct {
		name string
		q    Query
		want int
	}{
		{"no filter", Query{}, 3},
		{"pay_min 13jt", Query{PayMin: i64(13_000_000)}, 1},
		{"pay_max 13jt", Query{PayMax: i64(13_000_000)}, 1},
		{"remote only", Query{Remote: &trueVal}, 2},
		{"location jakarta", Query{Location: "Jakarta"}, 2},
		{"employment contract", Query{EmploymentType: "contract"}, 1},
		{"source glints", Query{Source: "glints"}, 1},
		{"skills go", Query{Skills: []string{"Go"}}, 1},
		{"skills react+typescript", Query{Skills: []string{"React", "TypeScript"}}, 1},
		{"max_yoe 3", Query{MaxYoE: iptr(3)}, 2}, // yoe 2 and the unspecified one
		{"posted after +36h", Query{PostedAfter: tptr(base.Add(36 * time.Hour))}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countFor(t, c.q); got != c.want {
				t.Errorf("%s: got %d, want %d", c.name, got, c.want)
			}
		})
	}
}

// AC-FEED-3: free-text search matches title and company.
func TestSearchTitleAndCompany(t *testing.T) {
	if got := countFor(t, Query{Search: "engineer"}); got != 2 {
		t.Errorf("title search: got %d, want 2", got)
	}
	if got := countFor(t, Query{Search: "traveloka"}); got != 1 {
		t.Errorf("company search: got %d, want 1", got)
	}
	if got := countFor(t, Query{Search: "nonexistent"}); got != 0 {
		t.Errorf("no-match search: got %d, want 0", got)
	}
}

// AC-FEED-1: paginated cards, newest first.
func TestPaginationOrder(t *testing.T) {
	ix := NewIndex(seed())
	page1 := ix.Search(Query{Limit: 2, Offset: 0})
	if page1.Total != 3 || len(page1.Jobs) != 2 {
		t.Fatalf("page1 total=%d len=%d", page1.Total, len(page1.Jobs))
	}
	// Newest posting first (Data Analyst at +48h).
	if page1.Jobs[0].Title != "Data Analyst" {
		t.Errorf("first card = %q, want newest 'Data Analyst'", page1.Jobs[0].Title)
	}
	page2 := ix.Search(Query{Limit: 2, Offset: 2})
	if len(page2.Jobs) != 1 {
		t.Fatalf("page2 len=%d, want 1", len(page2.Jobs))
	}
}

func TestPaginationUsesDedupKeyForEqualDateAndTitle(t *testing.T) {
	posted := base
	ix := NewIndex([]ingest.Job{
		{DedupKey: "z", Title: "Same Role", PostedAt: &posted},
		{DedupKey: "a", Title: "Same Role", PostedAt: &posted},
	})
	for offset, want := range []string{"a", "z"} {
		page := ix.Search(Query{Limit: 1, Offset: offset})
		if len(page.Jobs) != 1 || page.Jobs[0].DedupKey != want {
			t.Fatalf("offset %d = %#v, want %q", offset, page.Jobs, want)
		}
	}
}

// AC-FEED-1: response carries stated pay, labeled, and NO estimated field.
func TestFeedResponseNoEstimatedField(t *testing.T) {
	svc := NewService(stubProvider{seed()})
	rr := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(strings.ToLower(body), "estimat") {
		t.Fatalf("feed response must not contain an estimated pay field:\n%s", body)
	}

	var resp feedResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	// Stated salary labeled Indonesian-style; unstated is empty + stated=false.
	var backend, analyst *card
	for i := range resp.Jobs {
		switch resp.Jobs[i].Title {
		case "Senior Backend Engineer":
			backend = &resp.Jobs[i]
		case "Data Analyst":
			analyst = &resp.Jobs[i]
		}
	}
	if backend == nil || backend.Salary.Label != "Rp 15.000.000 - Rp 25.000.000" || !backend.Salary.Stated {
		t.Errorf("backend salary card wrong: %+v", backend.Salary)
	}
	if analyst == nil || analyst.Salary.Stated || analyst.Salary.Label != "" {
		t.Errorf("unstated salary should be empty/stated=false: %+v", analyst.Salary)
	}
}

func TestFeedHTTPFilters(t *testing.T) {
	svc := NewService(stubProvider{seed()})
	rr := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed?remote=true&location=Jakarta", nil))
	var resp feedResp
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 1 || resp.Jobs[0].Title != "Data Analyst" {
		t.Fatalf("combined filter wrong: total=%d", resp.Total)
	}
}

func TestFeedHTTPBadParam(t *testing.T) {
	svc := NewService(stubProvider{seed()})
	rr := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed?pay_min=abc", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestFeedHTTPRejectsInvalidNumericFilters(t *testing.T) {
	for _, query := range []string{
		"pay_min=-1",
		"pay_max=-1",
		"max_yoe=-1",
		"pay_min=20000000&pay_max=10000000",
	} {
		t.Run(query, func(t *testing.T) {
			svc := NewService(stubProvider{seed()})
			rr := httptest.NewRecorder()
			svc.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed?"+query, nil))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestFeedHTTPPreservesPaginationNormalization(t *testing.T) {
	svc := NewService(stubProvider{seed()})
	rr := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/feed?limit=1000&offset=-3", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp feedResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Limit != 100 || resp.Offset != 0 {
		t.Fatalf("pagination = limit %d offset %d, want 100/0", resp.Limit, resp.Offset)
	}
}
