package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type rewriteTransport struct {
	target *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func rewriteClient(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return &http.Client{Transport: rewriteTransport{target: target}}
}

func TestExtractCandidate(t *testing.T) {
	tests := []struct {
		platform Platform
		raw      string
		want     Candidate
		ok       bool
	}{
		{Greenhouse, "https://boards.greenhouse.io/xendit", Candidate{Slug: "xendit", Company: "Xendit"}, true},
		{Greenhouse, "https://job-boards.greenhouse.io/foo?gh_jid=1", Candidate{Slug: "foo", Company: "Foo"}, true},
		{Lever, "https://jobs.lever.co/palantir/abc", Candidate{Slug: "palantir", Company: "Palantir"}, true},
		{Workable, "https://apply.workable.com/danas/j/123", Candidate{Slug: "danas", Company: "Danas"}, true},
		{Ashby, "https://jobs.ashbyhq.com/reku", Candidate{Slug: "reku", Company: "Reku"}, true},
		{SmartRecruiters, "https://jobs.smartrecruiters.com/Cermati/744", Candidate{Slug: "Cermati", Company: "Cermati"}, true},
		{Workday, "https://aia.wd3.myworkdayjobs.com/en-US/external/job/Jakarta/Head_JR-1", Candidate{Slug: "aia", Host: "aia.wd3.myworkdayjobs.com", Tenant: "aia", Site: "external", Company: "Aia"}, true},
		{Workday, "https://tiketdotcom.wd3.myworkdayjobs.com/Tiket_Careers", Candidate{Slug: "tiketdotcom", Host: "tiketdotcom.wd3.myworkdayjobs.com", Tenant: "tiketdotcom", Site: "Tiket_Careers", Company: "Tiketdotcom"}, true},
		{Greenhouse, "https://example.com/xendit", Candidate{}, false},
		{Greenhouse, "not a url", Candidate{}, false},
		{Workday, "https://example.com/en-US/x", Candidate{}, false},
		{Workday, "https://alignmenthealthcare5.impl-wd12.myworkdayjobs.com/ahc_external", Candidate{}, false},
	}
	for _, tc := range tests {
		got, ok := ExtractCandidate(tc.platform, tc.raw)
		if ok != tc.ok {
			t.Fatalf("%s %q: ok=%v, want %v", tc.platform, tc.raw, ok, tc.ok)
		}
		if !ok {
			continue
		}
		tc.want.Platform = tc.platform
		if got != tc.want {
			t.Fatalf("%s %q:\n got  %+v\n want %+v", tc.platform, tc.raw, got, tc.want)
		}
	}
}

func TestFetchCDXParsesAndDeduplicates(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`[["original"],["https://boards.greenhouse.io/a"],["https://boards.greenhouse.io/a"],["https://boards.greenhouse.io/b"]]`))
	}))
	t.Cleanup(srv.Close)

	d := NewDiscoverer(srv.Client())
	d.cdxBase = srv.URL
	urls, err := d.FetchCDX(context.Background(), "boards.greenhouse.io/*", 10)
	if err != nil {
		t.Fatalf("fetch cdx: %v", err)
	}
	want := []string{"https://boards.greenhouse.io/a", "https://boards.greenhouse.io/b"}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("urls=%v, want %v", urls, want)
	}
	if gotQuery.Get("fl") != "original" || gotQuery.Get("output") != "json" {
		t.Fatalf("unexpected cdx query: %v", gotQuery)
	}
}

func TestValidateUsesPublicAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/boards/good/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jobs":[]}`))
		case strings.Contains(r.URL.Path, "/wday/cxs/good/"):
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"total":0,"jobPostings":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	d := NewDiscoverer(rewriteClient(t, srv))
	if !d.Validate(context.Background(), Candidate{Platform: Greenhouse, Slug: "good"}) {
		t.Fatal("live greenhouse board should validate")
	}
	if d.Validate(context.Background(), Candidate{Platform: Greenhouse, Slug: "dead"}) {
		t.Fatal("dead greenhouse board must not validate")
	}
	live := Candidate{Platform: Workday, Host: "good.wd3.myworkdayjobs.com", Tenant: "good", Site: "Careers"}
	if !d.Validate(context.Background(), live) {
		t.Fatal("live workday board should validate")
	}
}

func TestRunFiltersByIndonesiaRelevance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cdx"):
			_, _ = w.Write([]byte(`[["original"],["https://boards.greenhouse.io/jkt"],["https://boards.greenhouse.io/usa"]]`))
		case strings.Contains(r.URL.Path, "/boards/jkt/"):
			_, _ = w.Write([]byte(`{"jobs":[{"location":{"name":"Jakarta, Indonesia"}}]}`))
		case strings.Contains(r.URL.Path, "/boards/usa/"):
			_, _ = w.Write([]byte(`{"jobs":[{"location":{"name":"New York, USA"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	run := func(allowGlobal bool) []string {
		dir := t.TempDir()
		if _, err := Run(context.Background(), Config{
			Platforms:   []Platform{Greenhouse},
			Limit:       100,
			OutDir:      dir,
			Write:       true,
			AllowGlobal: allowGlobal,
			Client:      rewriteClient(t, srv),
			CDXBase:     srv.URL + "/cdx",
		}); err != nil {
			t.Fatalf("run: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "greenhouse.json"))
		if err != nil {
			t.Fatalf("read catalog: %v", err)
		}
		var entries []map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatalf("decode catalog: %v", err)
		}
		var slugs []string
		for _, e := range entries {
			slugs = append(slugs, e["slug"])
		}
		return slugs
	}

	if got := run(false); !reflect.DeepEqual(got, []string{"jkt"}) {
		t.Fatalf("default run slugs=%v, want [jkt] (US board dropped)", got)
	}
	if got := run(true); !reflect.DeepEqual(got, []string{"jkt", "usa"}) {
		t.Fatalf("allow-global run slugs=%v, want [jkt usa]", got)
	}
}

func TestMergeEntriesDeduplicatesAndSorts(t *testing.T) {
	existing := []map[string]string{
		{"slug": "b", "company": "B"},
		{"slug": "b", "company": "duplicate"},
		{"slug": "a", "company": "A"},
	}
	candidates := []Candidate{
		{Platform: Greenhouse, Slug: "c", Company: "C"},
		{Platform: Greenhouse, Slug: "a", Company: "A again"},
	}
	merged, added := MergeEntries(Greenhouse, existing, candidates)
	if added != 1 {
		t.Fatalf("added=%d, want 1", added)
	}
	var keys []string
	for _, entry := range merged {
		keys = append(keys, entry["slug"])
	}
	if !reflect.DeepEqual(keys, []string{"a", "b", "c"}) {
		t.Fatalf("keys=%v, want [a b c]", keys)
	}
}

func TestRunMergesValidatedCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cdx"):
			_, _ = w.Write([]byte(`[["original"],["https://boards.greenhouse.io/good"],["https://boards.greenhouse.io/dead"],["https://boards.greenhouse.io/good"]]`))
		case strings.Contains(r.URL.Path, "/boards/good/"):
			_, _ = w.Write([]byte(`{"jobs":[{"location":{"name":"Jakarta, Indonesia"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "greenhouse.json"),
		[]byte(`[{"slug":"existing","company":"Existing"}]`), 0o644); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	report, err := Run(context.Background(), Config{
		Platforms: []Platform{Greenhouse},
		Limit:     100,
		OutDir:    dir,
		Write:     true,
		Client:    rewriteClient(t, srv),
		CDXBase:   srv.URL + "/cdx",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	pr := report.Platforms[Greenhouse]
	if pr.Err != nil {
		t.Fatalf("platform error: %v", pr.Err)
	}
	if pr.Validated != 1 || pr.Added != 1 || pr.Total != 2 {
		t.Fatalf("report=%+v, want validated=1 added=1 total=2", pr)
	}

	data, err := os.ReadFile(filepath.Join(dir, "greenhouse.json"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var entries []map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	var slugs []string
	for _, entry := range entries {
		slugs = append(slugs, entry["slug"])
	}
	if !reflect.DeepEqual(slugs, []string{"existing", "good"}) {
		t.Fatalf("slugs=%v, want [existing good]", slugs)
	}
}
