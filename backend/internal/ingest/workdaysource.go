package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	workdayPageSize     = 20
	workdayDefaultLimit = 100
)

// WorkdayCompany is one curated Workday board. Workday's public careers site is
// served per-tenant from `{Host}/wday/cxs/{Tenant}/{Site}/jobs`, where Host is
// usually `{tenant}.wdN.myworkdayjobs.com`.
type WorkdayCompany struct {
	Host    string `json:"host"`
	Tenant  string `json:"tenant"`
	Site    string `json:"site"`
	Company string `json:"company"`
}

// WorkdaySource harvests one public Workday board. It is Tier 2: the CXS API is
// unauthenticated and returns the same postings the public careers page shows.
// A source is created per tenant/site so a dead board trips only its own
// circuit breaker.
type WorkdaySource struct {
	company  WorkdayCompany
	endpoint string
	limit    int
	client   *http.Client
}

// NewWorkdaySource builds a production Workday source.
func NewWorkdaySource(c WorkdayCompany, client *http.Client) *WorkdaySource {
	return NewWorkdaySourceWithEndpoint(c, workdayEndpoint(c), client)
}

// NewWorkdaySourceWithEndpoint builds a fixture-backed source with an explicit
// endpoint; the endpoint is injectable for offline tests.
func NewWorkdaySourceWithEndpoint(c WorkdayCompany, endpoint string, client *http.Client) *WorkdaySource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &WorkdaySource{
		company:  c,
		endpoint: strings.TrimRight(endpoint, "/"),
		limit:    workdayDefaultLimit,
		client:   client,
	}
}

func workdayEndpoint(c WorkdayCompany) string {
	host := strings.TrimRight(strings.TrimSpace(c.Host), "/")
	return fmt.Sprintf("https://%s/wday/cxs/%s/%s/jobs",
		host, url.PathEscape(strings.TrimSpace(c.Tenant)), url.PathEscape(strings.TrimSpace(c.Site)))
}

func (w *WorkdaySource) ID() string {
	return fmt.Sprintf("workday-%s-%s", sanitizeID(w.company.Tenant), sanitizeID(w.company.Site))
}

func (w *WorkdaySource) Tier() Tier { return Tier2 }

// Fetch pages through the board's job listings. Only the list endpoint is
// called; descriptions would require one extra request per posting, which the
// Jabodetabek-first filter makes wasteful (most global postings are dropped).
func (w *WorkdaySource) Fetch(ctx context.Context) ([]RawJob, error) {
	if w.endpoint == "" {
		return nil, fmt.Errorf("ingest: %s endpoint is empty", w.ID())
	}
	if strings.TrimSpace(w.company.Host) == "" || strings.TrimSpace(w.company.Tenant) == "" {
		return nil, fmt.Errorf("ingest: %s is missing Workday host/tenant", w.ID())
	}

	var jobs []RawJob
	for offset := 0; offset < w.limit; offset += workdayPageSize {
		page, total, err := w.fetchPage(ctx, offset)
		if err != nil {
			return nil, err
		}
		for _, posting := range page {
			jobs = append(jobs, w.raw(posting))
		}
		if len(page) == 0 || offset+workdayPageSize >= total || len(jobs) >= w.limit {
			break
		}
	}
	if len(jobs) > w.limit {
		jobs = jobs[:w.limit]
	}
	return jobs, nil
}

func (w *WorkdaySource) fetchPage(ctx context.Context, offset int) ([]workdayPosting, int, error) {
	body := workdayRequest{
		AppliedFacets: map[string]any{},
		Limit:         workdayPageSize,
		Offset:        offset,
		SearchText:    "",
	}
	payload, err := postJSON(ctx, w.client, w.ID(), w.endpoint, body)
	if err != nil {
		return nil, 0, err
	}
	var response workdayResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, 0, fmt.Errorf("ingest: decode %s: %w", w.ID(), err)
	}
	return response.JobPostings, response.Total, nil
}

func (w *WorkdaySource) raw(posting workdayPosting) RawJob {
	location := collapse(posting.LocationsText)
	if detectRemote(location) {
		location = "Remote"
	}
	return RawJob{
		Source:    "workday",
		SourceURL: w.publicURL(posting.ExternalPath),
		Title:     posting.Title,
		Company:   firstNonEmpty(w.company.Company, humanizeSlug(w.company.Tenant)),
		Location:  location,
		Remote:    detectRemote(posting.LocationsText),
	}
}

func (w *WorkdaySource) publicURL(externalPath string) string {
	path := strings.TrimSpace(externalPath)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	host := strings.TrimRight(strings.TrimSpace(w.company.Host), "/")
	return fmt.Sprintf("https://%s/en-US/%s%s", host, strings.TrimSpace(w.company.Site), path)
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func postJSON(ctx context.Context, client *http.Client, id, endpoint string, body any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ingest: encode request for %s: %w", id, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("ingest: build request for %s: %w", id, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned status %d", id, resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBody))
	if err != nil {
		return nil, fmt.Errorf("ingest: read %s: %w", id, err)
	}
	return payload, nil
}

type workdayRequest struct {
	AppliedFacets map[string]any `json:"appliedFacets"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	SearchText    string         `json:"searchText"`
}

type workdayResponse struct {
	Total       int              `json:"total"`
	JobPostings []workdayPosting `json:"jobPostings"`
}

type workdayPosting struct {
	Title         string   `json:"title"`
	ExternalPath  string   `json:"externalPath"`
	LocationsText string   `json:"locationsText"`
	PostedOn      string   `json:"postedOn"`
	BulletFields  []string `json:"bulletFields"`
}
