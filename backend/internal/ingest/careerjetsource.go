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

// Careerjet's current partner API uses HTTPS and Basic authentication. The
// value is called an affiliate id in the product configuration because that is
// how Careerjet describes the free publisher credential.
const careerjetSearchURL = "https://search.api.careerjet.net/v4/query"

const careerjetDefaultLimit = 25

// CareerjetSource is a keyed Tier 1 source for Indonesian listings. It is not
// registered when the affiliate id is absent, so local and production installs
// without a partner account degrade cleanly.
type CareerjetSource struct {
	id      string
	baseURL string
	affid   string
	limit   int
	client  *http.Client
}

// NewCareerjetSource builds the production source with its default cap.
func NewCareerjetSource(affid string, client *http.Client) *CareerjetSource {
	return NewCareerjetSourceWithURLAndLimit("careerjet", careerjetSearchURL, affid, careerjetDefaultLimit, client)
}

// NewCareerjetSourceWithLimit builds the production source with a per-run cap.
func NewCareerjetSourceWithLimit(affid string, limit int, client *http.Client) *CareerjetSource {
	return NewCareerjetSourceWithURLAndLimit("careerjet", careerjetSearchURL, affid, limit, client)
}

// NewCareerjetSourceWithURL builds a fixture-backed source. The explicit URL
// keeps network behavior mockable without changing production configuration.
func NewCareerjetSourceWithURL(id, baseURL, affid string, client *http.Client) *CareerjetSource {
	return NewCareerjetSourceWithURLAndLimit(id, baseURL, affid, careerjetDefaultLimit, client)
}

// NewCareerjetSourceWithURLAndLimit builds a fixture-backed source with a cap.
func NewCareerjetSourceWithURLAndLimit(id, baseURL, affid string, limit int, client *http.Client) *CareerjetSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if limit <= 0 {
		limit = careerjetDefaultLimit
	}
	return &CareerjetSource{
		id:      id,
		baseURL: strings.TrimRight(baseURL, "/"),
		affid:   strings.TrimSpace(affid),
		limit:   limit,
		client:  client,
	}
}

func (c *CareerjetSource) ID() string { return c.id }

func (c *CareerjetSource) Tier() Tier { return Tier1 }

// Fetch queries Careerjet's Indonesian locale, paging until the configured cap
// or the API's result set is exhausted.
func (c *CareerjetSource) Fetch(ctx context.Context) ([]RawJob, error) {
	if c.affid == "" {
		return nil, fmt.Errorf("ingest: %s requires an affiliate id", c.id)
	}

	jobs := make([]RawJob, 0, min(c.limit, careerjetPageSize))
	pageSize := min(c.limit, careerjetPageSize)
	for page := 1; len(jobs) < c.limit && page <= 10; page++ {
		body, err := c.fetchPage(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, item := range body.Jobs {
			if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" {
				continue
			}
			jobs = append(jobs, c.toRaw(item))
			if len(jobs) == c.limit {
				break
			}
		}
		if len(body.Jobs) == 0 || len(body.Jobs) < pageSize || (body.Pages > 0 && page >= body.Pages) {
			break
		}
	}
	return jobs, nil
}

func (c *CareerjetSource) fetchPage(ctx context.Context, page, pageSize int) (careerjetResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return careerjetResponse{}, fmt.Errorf("ingest: parse careerjet url: %w", err)
	}
	q := u.Query()
	q.Set("locale_code", "id_ID")
	q.Set("location", "Indonesia")
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	q.Set("sort", "date")
	q.Set("user_ip", "127.0.0.1")
	q.Set("user_agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return careerjetResponse{}, fmt.Errorf("ingest: build careerjet request: %w", err)
	}
	req.SetBasicAuth(c.affid, "")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")

	resp, err := c.client.Do(req)
	if err != nil {
		return careerjetResponse{}, fmt.Errorf("ingest: fetch %s: %w", c.id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return careerjetResponse{}, fmt.Errorf("ingest: %s returned status %d", c.id, resp.StatusCode)
	}

	var body careerjetResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return careerjetResponse{}, fmt.Errorf("ingest: decode %s: %w", c.id, err)
	}
	if body.Type != "" && !strings.EqualFold(body.Type, "JOBS") {
		return careerjetResponse{}, fmt.Errorf("ingest: %s returned response type %q", c.id, body.Type)
	}
	return body, nil
}

func (c *CareerjetSource) toRaw(j careerjetJob) RawJob {
	requirements := make([]string, 0, 1)
	if description := htmlToPlainText(j.Description); description != "" {
		requirements = append(requirements, description)
	}
	return RawJob{
		Source:         c.id,
		SourceURL:      strings.TrimSpace(j.URL),
		Title:          strings.TrimSpace(j.Title),
		Company:        strings.TrimSpace(j.Company),
		Location:       strings.TrimSpace(j.Locations),
		SalaryText:     careerjetSalaryText(j),
		EmploymentType: careerjetEmploymentType(j.ContractType),
		Requirements:   requirements,
		PostedAt:       parseSourceTime(j.Date),
	}
}

func careerjetEmploymentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "p":
		return "full_time"
	case "c":
		return "contract"
	case "t":
		return "temporary"
	case "i":
		return "internship"
	default:
		return value
	}
}

func careerjetSalaryText(j careerjetJob) string {
	if !strings.EqualFold(strings.TrimSpace(j.SalaryCurrencyCode), "IDR") {
		return ""
	}
	if strings.TrimSpace(j.Salary) != "" {
		return trustedSalaryText(j.Salary)
	}
	min, max := j.SalaryMin, j.SalaryMax
	switch {
	case min > 0 && max > 0:
		return fmt.Sprintf("Rp %g - Rp %g", min, max)
	case min > 0:
		return fmt.Sprintf("Rp %g", min)
	case max > 0:
		return fmt.Sprintf("Rp %g", max)
	default:
		return ""
	}
}

func parseSourceTime(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC1123,
		time.RFC1123Z,
		"Mon,02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed
		}
	}
	return nil
}

const careerjetPageSize = 100

type careerjetResponse struct {
	Type  string         `json:"type"`
	Hits  int            `json:"hits"`
	Pages int            `json:"pages"`
	Jobs  []careerjetJob `json:"jobs"`
}

type careerjetJob struct {
	Title              string  `json:"title"`
	Company            string  `json:"company"`
	Date               string  `json:"date"`
	Description        string  `json:"description"`
	Locations          string  `json:"locations"`
	Salary             string  `json:"salary"`
	SalaryCurrencyCode string  `json:"salary_currency_code"`
	SalaryMin          float64 `json:"salary_min"`
	SalaryMax          float64 `json:"salary_max"`
	ContractType       string  `json:"contract_type"`
	URL                string  `json:"url"`
}
