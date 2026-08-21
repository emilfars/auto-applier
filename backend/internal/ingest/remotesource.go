package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const remoteDefaultLimit = 100

// RemotiveSource is a keyless, remote-only public board source. Worldwide
// roles are deliberately retained because remote roles outside Indonesia are
// allowed by the sourcing-expansion decision.
type RemotiveSource struct {
	id      string
	baseURL string
	limit   int
	client  *http.Client
}

func NewRemotiveSource(limit int, client *http.Client) *RemotiveSource {
	return NewRemotiveSourceWithURL("remotive", "https://remotive.com/api/remote-jobs", limit, client)
}

func NewRemotiveSourceWithURL(id, baseURL string, limit int, client *http.Client) *RemotiveSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if limit <= 0 {
		limit = remoteDefaultLimit
	}
	return &RemotiveSource{id: id, baseURL: strings.TrimRight(baseURL, "/"), limit: limit, client: client}
}

func (r *RemotiveSource) ID() string { return r.id }
func (r *RemotiveSource) Tier() Tier { return Tier2 }

func (r *RemotiveSource) Fetch(ctx context.Context) ([]RawJob, error) {
	u, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, fmt.Errorf("ingest: parse %s url: %w", r.id, err)
	}
	q := u.Query()
	q.Set("limit", strconv.Itoa(min(r.limit, remoteDefaultLimit)))
	u.RawQuery = q.Encode()
	payload, err := fetchJSON(ctx, r.client, r.id, u.String())
	if err != nil {
		return nil, err
	}
	var response remotiveResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", r.id, err)
	}
	jobs := make([]RawJob, 0, min(len(response.Jobs), r.limit))
	for _, item := range response.Jobs {
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		jobs = append(jobs, RawJob{
			Source:         r.id,
			SourceURL:      item.URL,
			Title:          item.Title,
			Company:        item.CompanyName,
			Location:       "Remote",
			Remote:         true,
			SalaryText:     trustedSalaryText(item.Salary),
			EmploymentType: item.JobType,
			Requirements:   remoteRequirements(item.Description, item.Tags),
			PostedAt:       parseSourceTime(item.PublicationDate),
		})
		if len(jobs) == r.limit {
			break
		}
	}
	return jobs, nil
}

// JobicySource is a keyless remote-only public board source.
type JobicySource struct {
	id      string
	baseURL string
	limit   int
	client  *http.Client
}

func NewJobicySource(limit int, client *http.Client) *JobicySource {
	return NewJobicySourceWithURL("jobicy", "https://jobicy.com/api/v2/remote-jobs", limit, client)
}

func NewJobicySourceWithURL(id, baseURL string, limit int, client *http.Client) *JobicySource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if limit <= 0 {
		limit = remoteDefaultLimit
	}
	return &JobicySource{id: id, baseURL: strings.TrimRight(baseURL, "/"), limit: limit, client: client}
}

func (j *JobicySource) ID() string { return j.id }
func (j *JobicySource) Tier() Tier { return Tier2 }

func (j *JobicySource) Fetch(ctx context.Context) ([]RawJob, error) {
	u, err := url.Parse(j.baseURL)
	if err != nil {
		return nil, fmt.Errorf("ingest: parse %s url: %w", j.id, err)
	}
	q := u.Query()
	q.Set("count", strconv.Itoa(min(j.limit, remoteDefaultLimit)))
	u.RawQuery = q.Encode()
	payload, err := fetchJSON(ctx, j.client, j.id, u.String())
	if err != nil {
		return nil, err
	}
	var response jobicyResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", j.id, err)
	}
	jobs := make([]RawJob, 0, min(len(response.Jobs), j.limit))
	for _, item := range response.Jobs {
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		jobs = append(jobs, RawJob{
			Source:         j.id,
			SourceURL:      item.URL,
			Title:          item.Title,
			Company:        item.CompanyName,
			Location:       "Remote",
			Remote:         true,
			SalaryText:     idSalaryRangeText(firstRaw(item.SalaryMin, item.AnnualSalaryMin), firstRaw(item.SalaryMax, item.AnnualSalaryMax), item.SalaryCurrency),
			Seniority:      item.JobLevel,
			EmploymentType: strings.Join(item.JobType, ", "),
			Requirements:   remoteRequirements(item.Description, nil),
			PostedAt:       parseSourceTime(item.PubDate),
		})
		if len(jobs) == j.limit {
			break
		}
	}
	return jobs, nil
}

// RemoteOKSource is a keyless source for RemoteOK's public JSON feed. The API
// starts with a legal-notice object, which is intentionally skipped.
type RemoteOKSource struct {
	id      string
	baseURL string
	limit   int
	client  *http.Client
}

func NewRemoteOKSource(limit int, client *http.Client) *RemoteOKSource {
	return NewRemoteOKSourceWithURL("remoteok", "https://remoteok.com/api", limit, client)
}

func NewRemoteOKSourceWithURL(id, baseURL string, limit int, client *http.Client) *RemoteOKSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if limit <= 0 {
		limit = remoteDefaultLimit
	}
	return &RemoteOKSource{id: id, baseURL: strings.TrimRight(baseURL, "/"), limit: limit, client: client}
}

func (r *RemoteOKSource) ID() string { return r.id }
func (r *RemoteOKSource) Tier() Tier { return Tier2 }

func (r *RemoteOKSource) Fetch(ctx context.Context) ([]RawJob, error) {
	payload, err := fetchJSON(ctx, r.client, r.id, r.baseURL)
	if err != nil {
		return nil, err
	}
	var response []remoteOKJob
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", r.id, err)
	}
	jobs := make([]RawJob, 0, min(len(response), r.limit))
	for _, item := range response {
		if strings.TrimSpace(item.Position) == "" || firstNonEmpty(item.URL, item.ApplyURL) == "" {
			continue
		}
		jobs = append(jobs, RawJob{
			Source:       r.id,
			SourceURL:    firstNonEmpty(item.URL, item.ApplyURL),
			Title:        item.Position,
			Company:      item.Company,
			Location:     "Remote",
			Remote:       true,
			Requirements: remoteRequirements(item.Description, item.Tags),
			PostedAt:     parseSourceTime(item.Date),
		})
		if len(jobs) == r.limit {
			break
		}
	}
	return jobs, nil
}

func fetchJSON(ctx context.Context, client *http.Client, id, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ingest: build request for %s: %w", id, err)
	}
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
	var payload json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", id, err)
	}
	return payload, nil
}

func trustedSalaryText(value string) string {
	if min, _ := ParseSalaryIDR(value); min != nil {
		return value
	}
	return ""
}

func idSalaryRangeText(minRaw, maxRaw json.RawMessage, currency string) string {
	if !strings.EqualFold(strings.TrimSpace(currency), "IDR") {
		return ""
	}
	min, minOK := parseJSONFloat(minRaw)
	max, maxOK := parseJSONFloat(maxRaw)
	switch {
	case minOK && maxOK:
		return fmt.Sprintf("Rp %g - Rp %g", min, max)
	case minOK:
		return fmt.Sprintf("Rp %g", min)
	case maxOK:
		return fmt.Sprintf("Rp %g", max)
	default:
		return ""
	}
}

func parseJSONFloat(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil && number > 0 {
		return number, true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		return number, err == nil && number > 0
	}
	return 0, false
}

func firstRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

func remoteRequirements(description string, tags []string) []string {
	requirements := make([]string, 0, 1+len(tags))
	if plain := htmlToPlainText(description); plain != "" {
		requirements = append(requirements, plain)
	}
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			requirements = append(requirements, tag)
		}
	}
	return requirements
}

type remotiveResponse struct {
	Jobs []remotiveJob `json:"jobs"`
}

type remotiveJob struct {
	Title           string   `json:"title"`
	URL             string   `json:"url"`
	CompanyName     string   `json:"company_name"`
	JobType         string   `json:"job_type"`
	PublicationDate string   `json:"publication_date"`
	Salary          string   `json:"salary"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
}

type jobicyResponse struct {
	Jobs []jobicyJob `json:"jobs"`
}

type jobicyJob struct {
	URL             string          `json:"url"`
	Title           string          `json:"jobTitle"`
	CompanyName     string          `json:"companyName"`
	JobType         []string        `json:"jobType"`
	JobLevel        string          `json:"jobLevel"`
	PubDate         string          `json:"pubDate"`
	Description     string          `json:"jobDescription"`
	SalaryMin       json.RawMessage `json:"salaryMin"`
	SalaryMax       json.RawMessage `json:"salaryMax"`
	AnnualSalaryMin json.RawMessage `json:"annualSalaryMin"`
	AnnualSalaryMax json.RawMessage `json:"annualSalaryMax"`
	SalaryCurrency  string          `json:"salaryCurrency"`
}

type remoteOKJob struct {
	URL         string   `json:"url"`
	ApplyURL    string   `json:"apply_url"`
	Position    string   `json:"position"`
	Company     string   `json:"company"`
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
