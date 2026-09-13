// Package discover grows the curated ATS board catalogs under
// internal/ingest/atscompanies. It queries the Wayback Machine CDX index for
// archived board URLs, extracts candidate slugs, validates each board against
// its public API, and merges the survivors into the catalog JSON files.
//
// The approach is informed by mherzog4/job-boards (MIT) — Wayback CDX
// discovery plus validation before adding a slug — re-implemented here against
// the project's existing Source endpoints. No third-party code is used.
package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Platform is one supported public ATS board family.
type Platform string

const (
	Greenhouse      Platform = "greenhouse"
	Lever           Platform = "lever"
	Workable        Platform = "workable"
	Ashby           Platform = "ashby"
	Workday         Platform = "workday"
	SmartRecruiters Platform = "smartrecruiters"
)

const (
	defaultCDXBase = "https://web.archive.org/cdx/search/cdx"
	maxCDXBody     = 8 << 20
)

// Platforms returns all supported platforms in catalog order.
func Platforms() []Platform {
	return []Platform{Greenhouse, Lever, Workable, Ashby, Workday, SmartRecruiters}
}

// Candidate is one discovered board. Workday uses the host/tenant/site triple;
// the other platforms use Slug.
type Candidate struct {
	Platform Platform
	Slug     string
	Host     string
	Tenant   string
	Site     string
	Company  string
}

// Key is the catalog identity used for de-duplication.
func (c Candidate) Key() string {
	if c.Platform == Workday {
		return c.Host + "/" + c.Site
	}
	return c.Slug
}

// CDXPatterns returns the Wayback URL patterns searched for a platform.
func CDXPatterns(p Platform) []string {
	switch p {
	case Greenhouse:
		return []string{"boards.greenhouse.io/*", "job-boards.greenhouse.io/*"}
	case Lever:
		return []string{"jobs.lever.co/*"}
	case Workable:
		return []string{"apply.workable.com/*"}
	case Ashby:
		return []string{"jobs.ashbyhq.com/*"}
	case Workday:
		return []string{"*.myworkdayjobs.com/*"}
	case SmartRecruiters:
		return []string{"jobs.smartrecruiters.com/*"}
	default:
		return nil
	}
}

// Discoverer queries CDX and validates boards. It is safe for concurrent use.
type Discoverer struct {
	client  *http.Client
	cdxBase string
}

// NewDiscoverer builds a Discoverer; a nil client uses a sensible default.
func NewDiscoverer(client *http.Client) *Discoverer {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Discoverer{client: client, cdxBase: defaultCDXBase}
}

// FetchCDX returns unique original URLs from the CDX index for one pattern.
func (d *Discoverer) FetchCDX(ctx context.Context, pattern string, limit int) ([]string, error) {
	base := d.cdxBase
	if base == "" {
		base = defaultCDXBase
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("discover: parse cdx base: %w", err)
	}
	q := u.Query()
	q.Set("url", pattern)
	q.Set("output", "json")
	q.Set("fl", "original")
	q.Set("collapse", "urlkey")
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("discover: build cdx request: %w", err)
	}
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover: cdx request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover: cdx status %d", resp.StatusCode)
	}

	var rows [][]string
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCDXBody)).Decode(&rows); err != nil {
		return nil, fmt.Errorf("discover: decode cdx: %w", err)
	}
	seen := map[string]bool{}
	var out []string
	for i, row := range rows {
		if i == 0 { // header row
			continue
		}
		if len(row) == 0 {
			continue
		}
		original := strings.TrimSpace(row[0])
		if original == "" || seen[original] {
			continue
		}
		seen[original] = true
		out = append(out, original)
	}
	return out, nil
}

// Validate reports whether a candidate board exists by calling its public API.
// A 200 response is required; a dead slug returns 404.
func (d *Discoverer) Validate(ctx context.Context, c Candidate) bool {
	method, endpoint, body, ok := validationSpec(c)
	if !ok {
		return false
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return resp.StatusCode == http.StatusOK
}

func validationSpec(c Candidate) (method, endpoint string, body []byte, ok bool) {
	switch c.Platform {
	case Greenhouse:
		if c.Slug == "" {
			return "", "", nil, false
		}
		return http.MethodGet, "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(c.Slug) + "/jobs", nil, true
	case Lever:
		if c.Slug == "" {
			return "", "", nil, false
		}
		return http.MethodGet, "https://api.lever.co/v0/postings/" + url.PathEscape(c.Slug) + "?mode=json", nil, true
	case Workable:
		if c.Slug == "" {
			return "", "", nil, false
		}
		return http.MethodGet, "https://apply.workable.com/api/v1/widget/accounts/" + url.PathEscape(c.Slug), nil, true
	case Ashby:
		if c.Slug == "" {
			return "", "", nil, false
		}
		return http.MethodGet, "https://api.ashbyhq.com/posting-api/job-board/" + url.PathEscape(c.Slug), nil, true
	case SmartRecruiters:
		if c.Slug == "" {
			return "", "", nil, false
		}
		return http.MethodGet, "https://api.smartrecruiters.com/v1/companies/" + url.PathEscape(c.Slug) + "/postings?limit=1", nil, true
	case Workday:
		if c.Host == "" || c.Tenant == "" || c.Site == "" {
			return "", "", nil, false
		}
		payload, _ := json.Marshal(map[string]any{
			"appliedFacets": map[string]any{},
			"limit":         1,
			"offset":        0,
			"searchText":    "",
		})
		endpoint := fmt.Sprintf("https://%s/wday/cxs/%s/%s/jobs", c.Host, url.PathEscape(c.Tenant), url.PathEscape(c.Site))
		return http.MethodPost, endpoint, payload, true
	default:
		return "", "", nil, false
	}
}

var localeRe = regexp.MustCompile(`^[a-z]{2}(-[a-zA-Z]{2,4})?$`)

const reservedPathSegment = "job"

// ExtractCandidate maps one archived board URL to a candidate. It returns false
// when the URL does not belong to the platform or lacks a usable slug.
func ExtractCandidate(p Platform, raw string) (Candidate, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return Candidate{}, false
	}
	host := strings.ToLower(u.Hostname())
	segments := pathSegments(u.Path)

	slugBoard := func(wantHost string) (Candidate, bool) {
		if host != wantHost || len(segments) == 0 {
			return Candidate{}, false
		}
		slug := segments[0]
		return Candidate{Platform: p, Slug: slug, Company: titleize(slug)}, true
	}

	switch p {
	case Greenhouse:
		if c, ok := slugBoard("boards.greenhouse.io"); ok {
			return c, true
		}
		return slugBoard("job-boards.greenhouse.io")
	case Lever:
		return slugBoard("jobs.lever.co")
	case Workable:
		return slugBoard("apply.workable.com")
	case Ashby:
		return slugBoard("jobs.ashbyhq.com")
	case SmartRecruiters:
		return slugBoard("jobs.smartrecruiters.com")
	case Workday:
		if !strings.HasSuffix(host, ".myworkdayjobs.com") {
			return Candidate{}, false
		}
		tenant := strings.SplitN(host, ".", 2)[0]
		site := firstNonLocaleSegment(segments)
		if tenant == "" || site == "" || site == reservedPathSegment {
			return Candidate{}, false
		}
		return Candidate{
			Platform: Workday,
			Slug:     tenant,
			Host:     host,
			Tenant:   tenant,
			Site:     site,
			Company:  titleize(tenant),
		}, true
	default:
		return Candidate{}, false
	}
}

func pathSegments(path string) []string {
	var out []string
	for _, part := range strings.Split(path, "/") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonLocaleSegment(segments []string) string {
	for _, segment := range segments {
		if localeRe.MatchString(segment) {
			continue
		}
		return segment
	}
	return ""
}

func titleize(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

// MergeEntries merges validated candidates into existing catalog entries,
// de-duplicating by identity and sorting for deterministic output. It returns
// the merged entries and how many candidates were added.
func MergeEntries(p Platform, existing []map[string]string, candidates []Candidate) ([]map[string]string, int) {
	seen := map[string]bool{}
	out := make([]map[string]string, 0, len(existing)+len(candidates))
	for _, entry := range existing {
		key := entryKey(p, entry)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	added := 0
	for _, c := range candidates {
		entry := candidateEntry(c)
		key := entryKey(p, entry)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
		added++
	}
	sort.Slice(out, func(i, j int) bool { return entryKey(p, out[i]) < entryKey(p, out[j]) })
	return out, added
}

func entryKey(p Platform, entry map[string]string) string {
	if p == Workday {
		if entry["host"] == "" || entry["site"] == "" {
			return ""
		}
		return entry["host"] + "/" + entry["site"]
	}
	return entry["slug"]
}

func candidateEntry(c Candidate) map[string]string {
	if c.Platform == Workday {
		return map[string]string{
			"host":    c.Host,
			"tenant":  c.Tenant,
			"site":    c.Site,
			"company": c.Company,
		}
	}
	return map[string]string{"slug": c.Slug, "company": c.Company}
}

// Config configures a discovery run.
type Config struct {
	Platforms []Platform
	Limit     int
	OutDir    string
	Write     bool
	Client    *http.Client
	CDXBase   string // override for tests
}

// PlatformReport summarizes one platform.
type PlatformReport struct {
	Discovered int
	Validated  int
	Added      int
	Total      int
	Err        error
}

// Report summarizes a run.
type Report struct {
	Platforms map[Platform]PlatformReport
}

// Run discovers and (optionally) writes the catalogs for the configured
// platforms. A failure on one platform is recorded and does not abort the run.
func Run(ctx context.Context, cfg Config) (Report, error) {
	if len(cfg.Platforms) == 0 {
		cfg.Platforms = Platforms()
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 200
	}
	if cfg.OutDir == "" {
		cfg.OutDir = "internal/ingest/atscompanies"
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return Report{}, fmt.Errorf("discover: create output dir: %w", err)
	}
	d := &Discoverer{client: cfg.Client, cdxBase: cfg.CDXBase}

	report := Report{Platforms: map[Platform]PlatformReport{}}
	for _, p := range cfg.Platforms {
		pr := PlatformReport{}
		existing, err := readCatalog(cfg.OutDir, p)
		if err != nil {
			pr.Err = err
			report.Platforms[p] = pr
			continue
		}

		seen := map[string]bool{}
		var candidates []Candidate
		for _, pattern := range CDXPatterns(p) {
			urls, err := d.FetchCDX(ctx, pattern, cfg.Limit)
			if err != nil && pr.Err == nil {
				pr.Err = err
			}
			pr.Discovered += len(urls)
			for _, rawURL := range urls {
				c, ok := ExtractCandidate(p, rawURL)
				if !ok || seen[c.Key()] {
					continue
				}
				seen[c.Key()] = true
				candidates = append(candidates, c)
			}
		}

		valid := d.validateAll(ctx, candidates)
		pr.Validated = len(valid)
		merged, added := MergeEntries(p, existing, valid)
		pr.Added = added
		pr.Total = len(merged)
		if cfg.Write {
			if err := writeCatalog(cfg.OutDir, p, merged); err != nil {
				pr.Err = err
			}
		}
		report.Platforms[p] = pr
	}
	return report, nil
}

func (d *Discoverer) validateAll(ctx context.Context, candidates []Candidate) []Candidate {
	const workers = 8
	ok := make([]bool, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			ok[i] = d.Validate(ctx, candidates[i])
		}(i)
	}
	wg.Wait()
	var valid []Candidate
	for i := range candidates {
		if ok[i] {
			valid = append(valid, candidates[i])
		}
	}
	return valid
}

func catalogPath(dir string, p Platform) string {
	return filepath.Join(dir, string(p)+".json")
}

func readCatalog(dir string, p Platform) ([]map[string]string, error) {
	data, err := os.ReadFile(catalogPath(dir, p))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discover: read %s catalog: %w", p, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var entries []map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("discover: decode %s catalog: %w", p, err)
	}
	return entries, nil
}

func writeCatalog(dir string, p Platform, entries []map[string]string) error {
	if entries == nil {
		entries = []map[string]string{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("discover: encode %s catalog: %w", p, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(catalogPath(dir, p), data, 0o644); err != nil {
		return fmt.Errorf("discover: write %s catalog: %w", p, err)
	}
	return nil
}
