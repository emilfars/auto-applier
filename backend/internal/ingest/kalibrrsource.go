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

// kalibrrSearchURL is Kalibrr's public job-board search endpoint. It returns
// JSON (no login wall), so it is an allowed Tier 2 source. Of the requested
// Tier 2 boards it is the only one that serves listings to a static client;
// Glints/Jobstreet/Indeed sit behind anti-bot/WAF challenges and are not
// ingested (see plan.md sourcing-reality note).
const kalibrrSearchURL = "https://www.kalibrr.com/kjs/job_board/search"

// KalibrrSource is a Tier 2 source backed by Kalibrr's public JSON search API.
// It pulls Indonesia listings and maps them to RawJob values. Salary is only
// carried through when the employer stated it in IDR — no currency conversion,
// no estimation (stated-pay-only policy).
type KalibrrSource struct {
	id      string
	baseURL string
	country string
	limit   int
	client  *http.Client
}

// NewKalibrrSource builds the production Kalibrr source pulling up to limit
// Indonesia listings. A nil client uses a default with a timeout.
func NewKalibrrSource(limit int, client *http.Client) *KalibrrSource {
	return NewKalibrrSourceWithURL("kalibrr", kalibrrSearchURL, limit, client)
}

// NewKalibrrSourceWithURL builds a Kalibrr source against an explicit endpoint,
// used by tests to point at a local fixture server.
func NewKalibrrSourceWithURL(id, baseURL string, limit int, client *http.Client) *KalibrrSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if limit <= 0 {
		limit = 25
	}
	return &KalibrrSource{id: id, baseURL: baseURL, country: "Indonesia", limit: limit, client: client}
}

func (k *KalibrrSource) ID() string { return k.id }

// Tier is always Tier 2 — a public board with no login wall.
func (k *KalibrrSource) Tier() Tier { return Tier2 }

// Fetch queries the search endpoint and returns one RawJob per listing.
func (k *KalibrrSource) Fetch(ctx context.Context) ([]RawJob, error) {
	u, err := url.Parse(k.baseURL)
	if err != nil {
		return nil, fmt.Errorf("ingest: parse kalibrr url: %w", err)
	}
	q := u.Query()
	q.Set("limit", strconv.Itoa(k.limit))
	q.Set("offset", "0")
	q.Set("country", k.country)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("ingest: build kalibrr request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AutoApplierBot/1.0)")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", k.id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned status %d", k.id, resp.StatusCode)
	}

	var body kalibrrResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", k.id, err)
	}

	jobs := make([]RawJob, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		if strings.TrimSpace(j.Name) == "" {
			continue
		}
		jobs = append(jobs, k.toRaw(j))
	}
	return jobs, nil
}

func (k *KalibrrSource) toRaw(j kalibrrJob) RawJob {
	company := j.CompanyName
	if company == "" {
		company = j.Company.Name
	}
	return RawJob{
		Source:         k.id,
		SourceURL:      kalibrrJobURL(j),
		Title:          strings.TrimSpace(j.Name),
		Company:        strings.TrimSpace(company),
		Location:       kalibrrLocation(j),
		Remote:         j.IsWorkFromHome,
		SalaryText:     kalibrrSalaryText(j),
		EmploymentType: strings.TrimSpace(j.Tenure),
		PostedAt:       kalibrrPostedAt(j.ActivationDate),
	}
}

// kalibrrLocation renders "City, Region" from the structured google_location,
// dropping empty parts. The normalizer keys the city off the text before the
// first comma.
func kalibrrLocation(j kalibrrJob) string {
	c := j.GoogleLocation.AddressComponents
	parts := make([]string, 0, 2)
	if s := strings.TrimSpace(c.City); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(c.Region); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// kalibrrSalaryText formats a stated IDR salary into text the normalizer can
// parse. Non-IDR or unshown salaries yield "" so the feed shows "not
// disclosed" rather than a converted/fabricated figure.
func kalibrrSalaryText(j kalibrrJob) string {
	if !j.SalaryShown || !strings.EqualFold(j.SalaryCurrency, "IDR") {
		return ""
	}
	switch {
	case j.MinimumSalary != nil && j.MaximumSalary != nil:
		return fmt.Sprintf("Rp %d - %d", int64(*j.MinimumSalary), int64(*j.MaximumSalary))
	case j.MaximumSalary != nil:
		return fmt.Sprintf("Rp %d", int64(*j.MaximumSalary))
	case j.MinimumSalary != nil:
		return fmt.Sprintf("Rp %d", int64(*j.MinimumSalary))
	}
	return ""
}

func kalibrrJobURL(j kalibrrJob) string {
	if j.Company.Code != "" && j.Slug != "" {
		return fmt.Sprintf("https://www.kalibrr.com/c/%s/jobs/%d/%s", j.Company.Code, j.ID, j.Slug)
	}
	return fmt.Sprintf("https://www.kalibrr.com/id-ID/job-board/%d", j.ID)
}

func kalibrrPostedAt(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}

// --- JSON shapes (only the fields we consume) ---

type kalibrrResponse struct {
	Jobs []kalibrrJob `json:"jobs"`
}

type kalibrrJob struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	CompanyName    string   `json:"company_name"`
	Tenure         string   `json:"tenure"`
	SalaryShown    bool     `json:"salary_shown"`
	MinimumSalary  *float64 `json:"minimum_salary"`
	MaximumSalary  *float64 `json:"maximum_salary"`
	SalaryCurrency string   `json:"salary_currency"`
	IsWorkFromHome bool     `json:"is_work_from_home"`
	ActivationDate string   `json:"activation_date"`
	Company        struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"company"`
	GoogleLocation struct {
		AddressComponents struct {
			City   string `json:"city"`
			Region string `json:"region"`
		} `json:"address_components"`
	} `json:"google_location"`
}
