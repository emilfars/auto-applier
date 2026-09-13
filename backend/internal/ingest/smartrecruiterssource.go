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

const (
	smartRecruitersPageSize     = 100
	smartRecruitersDefaultLimit = 100
)

// SmartRecruitersCompany is one curated SmartRecruiters board. Slug is the
// company identifier used by the public API (`/v1/companies/{slug}/postings`).
type SmartRecruitersCompany struct {
	Slug    string `json:"slug"`
	Company string `json:"company"`
}

// SmartRecruitersSource harvests one public SmartRecruiters board. It is
// Tier 2: the postings API is unauthenticated and mirrors the public careers
// page.
type SmartRecruitersSource struct {
	company  SmartRecruitersCompany
	endpoint string
	limit    int
	client   *http.Client
}

// NewSmartRecruitersSource builds a production SmartRecruiters source.
func NewSmartRecruitersSource(c SmartRecruitersCompany, client *http.Client) *SmartRecruitersSource {
	return NewSmartRecruitersSourceWithEndpoint(c, smartRecruitersEndpoint(c.Slug), client)
}

// NewSmartRecruitersSourceWithEndpoint builds a fixture-backed source with an
// explicit endpoint; the endpoint is injectable for offline tests.
func NewSmartRecruitersSourceWithEndpoint(c SmartRecruitersCompany, endpoint string, client *http.Client) *SmartRecruitersSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &SmartRecruitersSource{
		company:  c,
		endpoint: strings.TrimRight(endpoint, "/"),
		limit:    smartRecruitersDefaultLimit,
		client:   client,
	}
}

func smartRecruitersEndpoint(slug string) string {
	return "https://api.smartrecruiters.com/v1/companies/" + url.PathEscape(strings.TrimSpace(slug)) + "/postings"
}

func (s *SmartRecruitersSource) ID() string {
	return "smartrecruiters-" + sanitizeID(s.company.Slug)
}

func (s *SmartRecruitersSource) Tier() Tier { return Tier2 }

// Fetch pages through the board's postings. Only the list endpoint is called;
// descriptions would require one extra request per posting and most global
// postings are dropped by the Jabodetabek-first filter.
func (s *SmartRecruitersSource) Fetch(ctx context.Context) ([]RawJob, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("ingest: %s endpoint is empty", s.ID())
	}
	var jobs []RawJob
	for offset := 0; offset < s.limit; offset += smartRecruitersPageSize {
		page, total, err := s.fetchPage(ctx, offset)
		if err != nil {
			return nil, err
		}
		for _, posting := range page {
			jobs = append(jobs, s.raw(posting))
		}
		if len(page) == 0 || offset+smartRecruitersPageSize >= total || len(jobs) >= s.limit {
			break
		}
	}
	if len(jobs) > s.limit {
		jobs = jobs[:s.limit]
	}
	return jobs, nil
}

func (s *SmartRecruitersSource) fetchPage(ctx context.Context, offset int) ([]smartRecruitersPosting, int, error) {
	u, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, 0, fmt.Errorf("ingest: parse %s url: %w", s.ID(), err)
	}
	q := u.Query()
	q.Set("limit", strconv.Itoa(smartRecruitersPageSize))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()

	payload, err := fetchJSON(ctx, s.client, s.ID(), u.String())
	if err != nil {
		return nil, 0, err
	}
	var response smartRecruitersResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, 0, fmt.Errorf("ingest: decode %s: %w", s.ID(), err)
	}
	return response.Content, response.TotalFound, nil
}

func (s *SmartRecruitersSource) raw(posting smartRecruitersPosting) RawJob {
	location := joinNonEmpty(", ", posting.Location.City, posting.Location.Region, posting.Location.Country)
	remote := posting.Location.Remote || detectRemote(location) || strings.EqualFold(strings.TrimSpace(posting.Location.Country), "remote")
	if remote && location == "" {
		location = "Remote"
	}
	return RawJob{
		Source:         "smartrecruiters",
		SourceURL:      fmt.Sprintf("https://jobs.smartrecruiters.com/%s/%s", s.company.Slug, posting.ID),
		Title:          posting.Name,
		Company:        firstNonEmpty(posting.Company.Name, s.company.Company),
		Location:       location,
		Remote:         remote,
		EmploymentType: posting.TypeOfEmployment.Label,
		PostedAt:       parseSourceTime(posting.ReleasedDate),
	}
}

func joinNonEmpty(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, sep)
}

type smartRecruitersResponse struct {
	TotalFound int                      `json:"totalFound"`
	Content    []smartRecruitersPosting `json:"content"`
}

type smartRecruitersPosting struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ReleasedDate string `json:"releasedDate"`
	Ref          string `json:"ref"`
	Company      struct {
		Name string `json:"name"`
	} `json:"company"`
	Location struct {
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
		Remote  bool   `json:"remote"`
	} `json:"location"`
	TypeOfEmployment struct {
		Label string `json:"label"`
	} `json:"typeOfEmployment"`
}
