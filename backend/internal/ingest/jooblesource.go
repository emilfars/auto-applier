package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// joobleBaseURL is the Jooble partner API base. The API key is appended as a
// path segment (POST /{key}). Jooble is a partner aggregator feed, so it is a
// Tier 1 source. It requires a free API key; without JOOBLE_API_KEY set the
// source is simply not registered (graceful degradation, mirroring the
// CV-parser and Google-OAuth optional-dependency pattern).
const joobleBaseURL = "https://jooble.org/api"

// JoobleSource is a Tier 1 source backed by the Jooble partner API. It queries
// Indonesia listings and maps them to RawJob values. Salary text is only
// carried through when it looks IDR-denominated (stated-pay-only, no currency
// conversion).
type JoobleSource struct {
	id       string
	baseURL  string
	apiKey   string
	location string
	keywords string
	client   *http.Client
}

// NewJoobleSource builds the production Jooble source for Indonesia. A nil
// client uses a default with a timeout.
func NewJoobleSource(apiKey string, client *http.Client) *JoobleSource {
	return NewJoobleSourceWithURL("jooble", joobleBaseURL, apiKey, client)
}

// NewJoobleSourceWithURL builds a Jooble source against an explicit base URL,
// used by tests to point at a local fixture server.
func NewJoobleSourceWithURL(id, baseURL, apiKey string, client *http.Client) *JoobleSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &JoobleSource{
		id:       id,
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		location: "Indonesia",
		client:   client,
	}
}

func (j *JoobleSource) ID() string { return j.id }

// Tier is always Tier 1 — a partner aggregator feed.
func (j *JoobleSource) Tier() Tier { return Tier1 }

// Fetch POSTs the search query and returns one RawJob per listing. It returns
// an error if the API key is empty so a misconfigured source fails loudly
// rather than silently yielding nothing.
func (j *JoobleSource) Fetch(ctx context.Context) ([]RawJob, error) {
	if strings.TrimSpace(j.apiKey) == "" {
		return nil, fmt.Errorf("ingest: %s requires an API key", j.id)
	}

	reqBody, err := json.Marshal(joobleRequest{Keywords: j.keywords, Location: j.location})
	if err != nil {
		return nil, fmt.Errorf("ingest: marshal jooble request: %w", err)
	}
	endpoint := j.baseURL + "/" + j.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ingest: build jooble request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", j.id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned status %d", j.id, resp.StatusCode)
	}

	var body joobleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", j.id, err)
	}

	jobs := make([]RawJob, 0, len(body.Jobs))
	for _, r := range body.Jobs {
		if strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.Link) == "" {
			continue
		}
		jobs = append(jobs, j.toRaw(r))
	}
	return jobs, nil
}

func (j *JoobleSource) toRaw(r joobleJob) RawJob {
	return RawJob{
		Source:         j.id,
		SourceURL:      strings.TrimSpace(r.Link),
		Title:          strings.TrimSpace(r.Title),
		Company:        strings.TrimSpace(r.Company),
		Location:       strings.TrimSpace(r.Location),
		SalaryText:     joobleSalaryText(r.Salary),
		EmploymentType: strings.TrimSpace(r.Type),
		PostedAt:       joobleUpdatedAt(r.Updated),
	}
}

// joobleSalaryText keeps a salary only when it is IDR-denominated. Jooble
// returns free-text salaries in mixed currencies; carrying a non-IDR figure
// through the IDR parser would fabricate a wrong number, so anything without an
// IDR marker is dropped (feed shows "not disclosed").
func joobleSalaryText(s string) string {
	l := strings.ToLower(strings.TrimSpace(s))
	if l == "" {
		return ""
	}
	if strings.Contains(l, "rp") || strings.Contains(l, "idr") ||
		strings.Contains(l, "juta") || strings.Contains(l, "jt") {
		return s
	}
	return ""
}

func joobleUpdatedAt(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.9999999", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// --- JSON shapes ---

type joobleRequest struct {
	Keywords string `json:"keywords"`
	Location string `json:"location"`
}

type joobleResponse struct {
	TotalCount int         `json:"totalCount"`
	Jobs       []joobleJob `json:"jobs"`
}

type joobleJob struct {
	Title    string `json:"title"`
	Location string `json:"location"`
	Snippet  string `json:"snippet"`
	Salary   string `json:"salary"`
	Source   string `json:"source"`
	Type     string `json:"type"`
	Link     string `json:"link"`
	Company  string `json:"company"`
	Updated  string `json:"updated"`
}
