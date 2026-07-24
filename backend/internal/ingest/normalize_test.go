package ingest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// AC-SCR-2: a normalized job carries all required fields.
func TestNormalizePopulatesAllFields(t *testing.T) {
	posted := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	raw := RawJob{
		Source:         "jobstreet",
		SourceURL:      "https://jobstreet.co.id/job/123",
		Title:          "  Senior   Backend Engineer ",
		Company:        "PT Tokopedia",
		Location:       "Jakarta Selatan, DKI Jakarta",
		SalaryText:     "Rp 15.000.000 - Rp 25.000.000",
		Seniority:      "Senior Level",
		EmploymentType: "Full Time",
		Requirements:   []string{"Go", " ", "PostgreSQL"},
		PostedAt:       &posted,
	}
	j, err := Normalize(raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if j.Title != "Senior Backend Engineer" {
		t.Errorf("title = %q", j.Title)
	}
	if j.Company != "PT Tokopedia" || j.Location != "Jakarta Selatan, DKI Jakarta" {
		t.Errorf("company/location wrong: %+v", j)
	}
	if j.SalaryCurrency != "IDR" {
		t.Errorf("currency = %q, want IDR", j.SalaryCurrency)
	}
	if j.SalaryStatedMin == nil || *j.SalaryStatedMin != 15_000_000 ||
		j.SalaryStatedMax == nil || *j.SalaryStatedMax != 25_000_000 {
		t.Errorf("salary = %v..%v", ptr(j.SalaryStatedMin), ptr(j.SalaryStatedMax))
	}
	if j.EmploymentType != "full_time" {
		t.Errorf("employment_type = %q", j.EmploymentType)
	}
	if j.Seniority != "senior" {
		t.Errorf("seniority = %q", j.Seniority)
	}
	if !reflect.DeepEqual(j.Requirements, []string{"Go", "PostgreSQL"}) {
		t.Errorf("requirements = %v", j.Requirements)
	}
	if j.PostedAt == nil || !j.PostedAt.Equal(posted) {
		t.Errorf("posted_at = %v", j.PostedAt)
	}
	if j.DedupKey == "" {
		t.Error("dedup key empty")
	}
	if j.SourceURL != raw.SourceURL {
		t.Errorf("source_url = %q", j.SourceURL)
	}
}

func TestNormalizeRejectsIncomplete(t *testing.T) {
	cases := []RawJob{
		{SourceURL: "u", Title: "t", Company: "c"},              // no source
		{Source: "s", Title: "t", Company: "c"},                 // no url
		{Source: "s", SourceURL: "u", Company: "c"},             // no title
		{Source: "s", SourceURL: "u", Title: "t"},               // no company
		{Source: " ", SourceURL: "u", Title: "t", Company: "c"}, // blank source
	}
	for i, raw := range cases {
		if _, err := Normalize(raw); !errors.Is(err, ErrIncomplete) {
			t.Errorf("case %d: err = %v, want ErrIncomplete", i, err)
		}
	}
}

func TestNormalizeDetectsRemote(t *testing.T) {
	for _, loc := range []string{"Remote", "Jakarta (WFH)", "Kerja dari rumah"} {
		j, err := Normalize(RawJob{Source: "s", SourceURL: "u", Title: "Dev", Company: "c", Location: loc})
		if err != nil {
			t.Fatal(err)
		}
		if !j.Remote {
			t.Errorf("location %q should be remote", loc)
		}
	}
}

func TestDedupKeyCollapsesAcrossSources(t *testing.T) {
	a, _ := Normalize(RawJob{Source: "jobstreet", SourceURL: "https://a", Title: "Backend Engineer", Company: "PT Tokopedia", Location: "Jakarta Selatan, DKI"})
	b, _ := Normalize(RawJob{Source: "glints", SourceURL: "https://b", Title: "backend  engineer", Company: "pt tokopedia", Location: "Jakarta Selatan"})
	c, _ := Normalize(RawJob{Source: "glints", SourceURL: "https://c", Title: "Frontend Engineer", Company: "PT Tokopedia", Location: "Jakarta Selatan"})
	if a.DedupKey != b.DedupKey {
		t.Errorf("same job across sources should share a dedup key: %s vs %s", a.DedupKey, b.DedupKey)
	}
	if a.DedupKey == c.DedupKey {
		t.Error("different titles must not collide")
	}
}

func TestParseSalaryIDR(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	cases := []struct {
		in       string
		min, max *int64
	}{
		{"Rp 8.000.000 - Rp 12.000.000", i(8_000_000), i(12_000_000)},
		{"8 - 12 juta", i(8_000_000), i(12_000_000)},
		{"Rp5jt", i(5_000_000), i(5_000_000)},
		{"Rp 10.000.000", i(10_000_000), i(10_000_000)},
		{"8,5 - 10 juta", i(8_500_000), i(10_000_000)},
		{"Negotiable", nil, nil},
		{"Gaji kompetitif", nil, nil},
		{"", nil, nil},
	}
	for _, c := range cases {
		gotMin, gotMax := ParseSalaryIDR(c.in)
		if !eqPtr(gotMin, c.min) || !eqPtr(gotMax, c.max) {
			t.Errorf("ParseSalaryIDR(%q) = %v..%v, want %v..%v", c.in, ptr(gotMin), ptr(gotMax), ptr(c.min), ptr(c.max))
		}
	}
}

func ptr(p *int64) int64 {
	if p == nil {
		return -1
	}
	return *p
}
func eqPtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// --- registry (AC-SCR-2b) ---

type stubSource struct {
	id   string
	tier Tier
}

func (s stubSource) ID() string                              { return s.id }
func (s stubSource) Tier() Tier                              { return s.tier }
func (s stubSource) Fetch(context.Context) ([]RawJob, error) { return nil, nil }

// AC-SCR-2b: the registry accepts Tier 1/2 and excludes login-walled Tier 3.
func TestRegistryExcludesTier3(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(stubSource{"partner-api", Tier1}); err != nil {
		t.Fatalf("tier1 rejected: %v", err)
	}
	if err := r.Register(stubSource{"public-board", Tier2}); err != nil {
		t.Fatalf("tier2 rejected: %v", err)
	}
	if err := r.Register(stubSource{"linkedin", Tier3}); !errors.Is(err, ErrTierExcluded) {
		t.Fatalf("tier3 err = %v, want ErrTierExcluded", err)
	}
	if got := len(r.Sources()); got != 2 {
		t.Fatalf("registered = %d, want 2 (tier3 excluded)", got)
	}
}

func TestRegistryRejectsDuplicateAndEmpty(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(stubSource{"x", Tier1})
	if err := r.Register(stubSource{"x", Tier2}); err == nil {
		t.Error("duplicate id should be rejected")
	}
	if err := r.Register(stubSource{"", Tier1}); err == nil {
		t.Error("empty id should be rejected")
	}
}
