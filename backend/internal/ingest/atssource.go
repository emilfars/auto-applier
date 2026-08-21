package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ATSPlatform identifies one of the public, unauthenticated ATS job-board
// APIs. These are Tier 2 supplements, not login-walled portal scrapers.
type ATSPlatform string

const (
	ATSGreenhouse ATSPlatform = "greenhouse"
	ATSLever      ATSPlatform = "lever"
	ATSWorkable   ATSPlatform = "workable"
	ATSAshby      ATSPlatform = "ashby"
)

var atsSlugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ATSSource harvests one public company board. A source is created per slug so
// a deleted or mistyped company board trips only that slug's circuit breaker;
// healthy companies continue ingesting in the same pass.
type ATSSource struct {
	platform ATSPlatform
	slug     string
	company  string
	endpoint string
	client   *http.Client
}

// NewATSSource creates a production public-board source for one company slug.
func NewATSSource(platform ATSPlatform, slug string, client *http.Client) *ATSSource {
	return NewATSSourceWithURL(platform, slug, "", atsEndpoint(platform, slug), client)
}

// NewATSSourceWithURL creates a fixture-backed source using an explicit API
// endpoint. The endpoint is intentionally injectable for offline tests.
func NewATSSourceWithURL(platform ATSPlatform, slug, company, endpoint string, client *http.Client) *ATSSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.TrimSpace(company) == "" {
		company = humanizeSlug(slug)
	}
	return &ATSSource{
		platform: platform,
		slug:     strings.TrimSpace(slug),
		company:  strings.TrimSpace(company),
		endpoint: strings.TrimRight(endpoint, "/"),
		client:   client,
	}
}

func NewGreenhouseSource(slug string, client *http.Client) *ATSSource {
	return NewATSSource(ATSGreenhouse, slug, client)
}

func NewGreenhouseSourceWithURL(slug, endpoint string, client *http.Client) *ATSSource {
	return NewATSSourceWithURL(ATSGreenhouse, slug, "", endpoint, client)
}

func NewLeverSource(slug string, client *http.Client) *ATSSource {
	return NewATSSource(ATSLever, slug, client)
}

func NewLeverSourceWithURL(slug, endpoint string, client *http.Client) *ATSSource {
	return NewATSSourceWithURL(ATSLever, slug, "", endpoint, client)
}

func NewWorkableSource(slug string, client *http.Client) *ATSSource {
	return NewATSSource(ATSWorkable, slug, client)
}

func NewWorkableSourceWithURL(slug, endpoint string, client *http.Client) *ATSSource {
	return NewATSSourceWithURL(ATSWorkable, slug, "", endpoint, client)
}

func NewAshbySource(slug string, client *http.Client) *ATSSource {
	return NewATSSource(ATSAshby, slug, client)
}

func NewAshbySourceWithURL(slug, endpoint string, client *http.Client) *ATSSource {
	return NewATSSourceWithURL(ATSAshby, slug, "", endpoint, client)
}

func (a *ATSSource) ID() string {
	return fmt.Sprintf("ats-%s-%s", a.platform, a.slug)
}

func (a *ATSSource) Tier() Tier { return Tier2 }

func (a *ATSSource) Fetch(ctx context.Context) ([]RawJob, error) {
	if !atsSlugRe.MatchString(a.slug) {
		return nil, fmt.Errorf("ingest: invalid %s board slug %q", a.platform, a.slug)
	}
	if a.endpoint == "" {
		return nil, fmt.Errorf("ingest: %s board endpoint is empty", a.ID())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ingest: build request for %s: %w", a.ID(), err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", a.ID(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned status %d", a.ID(), resp.StatusCode)
	}

	var payload json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("ingest: decode %s: %w", a.ID(), err)
	}
	jobs, err := a.decode(payload)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (a *ATSSource) decode(payload json.RawMessage) ([]RawJob, error) {
	switch a.platform {
	case ATSGreenhouse:
		var response greenhouseResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			return nil, fmt.Errorf("ingest: decode %s response: %w", a.ID(), err)
		}
		jobs := make([]RawJob, 0, len(response.Jobs))
		for _, job := range response.Jobs {
			jobs = append(jobs, a.greenhouseRaw(job))
		}
		return jobs, nil
	case ATSLever:
		var response []leverJob
		if err := json.Unmarshal(payload, &response); err != nil {
			return nil, fmt.Errorf("ingest: decode %s response: %w", a.ID(), err)
		}
		jobs := make([]RawJob, 0, len(response))
		for _, job := range response {
			jobs = append(jobs, a.leverRaw(job))
		}
		return jobs, nil
	case ATSWorkable:
		var response workableResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			return nil, fmt.Errorf("ingest: decode %s response: %w", a.ID(), err)
		}
		jobs := make([]RawJob, 0, len(response.Jobs))
		for _, job := range response.Jobs {
			jobs = append(jobs, a.workableRaw(job, response.Name))
		}
		return jobs, nil
	case ATSAshby:
		var response ashbyResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			return nil, fmt.Errorf("ingest: decode %s response: %w", a.ID(), err)
		}
		jobs := make([]RawJob, 0, len(response.Jobs))
		for _, job := range response.Jobs {
			if job.IsListed != nil && !*job.IsListed {
				continue
			}
			jobs = append(jobs, a.ashbyRaw(job))
		}
		return jobs, nil
	default:
		return nil, fmt.Errorf("ingest: unsupported ATS platform %q", a.platform)
	}
}

func (a *ATSSource) greenhouseRaw(j greenhouseJob) RawJob {
	return RawJob{
		Source:       string(a.platform),
		SourceURL:    j.AbsoluteURL,
		Title:        j.Title,
		Company:      firstNonEmpty(j.CompanyName, a.company),
		Location:     j.Location.Name,
		SalaryText:   "",
		Requirements: descriptionRequirements(j.Content),
		PostedAt:     firstTime(j.FirstPublished, j.UpdatedAt),
	}
}

func (a *ATSSource) leverRaw(j leverJob) RawJob {
	location := firstNonEmpty(j.Categories.Location, strings.Join(j.Categories.AllLocations, ", "))
	remote := strings.Contains(strings.ToLower(j.WorkplaceType), "remote") || detectRemote(location)
	return RawJob{
		Source:         string(a.platform),
		SourceURL:      firstNonEmpty(j.HostedURL, j.ApplyURL),
		Title:          j.Text,
		Company:        a.company,
		Location:       location,
		Remote:         remote,
		SalaryText:     salaryRangeText(j.SalaryRange),
		EmploymentType: j.Categories.Commitment,
		Requirements:   descriptionRequirements(firstNonEmpty(j.DescriptionPlain, j.Description)),
		PostedAt:       unixMillisTime(j.CreatedAt),
	}
}

func (a *ATSSource) workableRaw(j workableJob, responseCompany string) RawJob {
	location, telecommuting := workableLocation(j.Location, j.Locations)
	remote := j.Telecommuting || telecommuting || detectRemote(location)
	return RawJob{
		Source:         string(a.platform),
		SourceURL:      firstNonEmpty(j.ApplicationURL, j.URL, j.Shortlink),
		Title:          j.Title,
		Company:        firstNonEmpty(responseCompany, a.company),
		Location:       location,
		Remote:         remote,
		SalaryText:     salaryRangeText(j.Salary),
		Seniority:      j.Experience,
		EmploymentType: j.EmploymentType,
		Requirements:   descriptionRequirements(firstNonEmpty(j.FullDescription, j.Description)),
		PostedAt:       firstTime(j.PublishedOn, j.CreatedAt),
	}
}

func (a *ATSSource) ashbyRaw(j ashbyJob) RawJob {
	location := j.Location
	remote := j.IsRemote || strings.EqualFold(j.WorkplaceType, "remote") || detectRemote(location)
	if remote {
		location = "Remote"
	}
	return RawJob{
		Source:         string(a.platform),
		SourceURL:      firstNonEmpty(j.JobURL, j.ApplyURL),
		Title:          j.Title,
		Company:        a.company,
		Location:       location,
		Remote:         remote,
		SalaryText:     trustedSalaryText(j.Compensation.SalarySummary),
		EmploymentType: j.EmploymentType,
		Requirements:   descriptionRequirements(firstNonEmpty(j.DescriptionPlain, j.DescriptionHTML)),
		PostedAt:       parseSourceTime(j.PublishedAt),
	}
}

func atsEndpoint(platform ATSPlatform, slug string) string {
	escaped := url.PathEscape(slug)
	switch platform {
	case ATSGreenhouse:
		return "https://boards-api.greenhouse.io/v1/boards/" + escaped + "/jobs?content=true"
	case ATSLever:
		return "https://api.lever.co/v0/postings/" + escaped + "?mode=json"
	case ATSWorkable:
		return "https://apply.workable.com/api/v1/widget/accounts/" + escaped + "?details=true"
	case ATSAshby:
		return "https://api.ashbyhq.com/posting-api/job-board/" + escaped + "?includeCompensation=true"
	default:
		return ""
	}
}

func humanizeSlug(slug string) string {
	return strings.Title(strings.ReplaceAll(strings.ReplaceAll(slug, "-", " "), "_", " "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func descriptionRequirements(raw string) []string {
	if plain := htmlToPlainText(raw); plain != "" {
		return []string{plain}
	}
	return nil
}

func firstTime(values ...string) *time.Time {
	for _, value := range values {
		if parsed := parseSourceTime(value); parsed != nil {
			return parsed
		}
	}
	return nil
}

func unixMillisTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	parsed := time.UnixMilli(value).UTC()
	return &parsed
}

func salaryRangeText(r salaryRange) string {
	currency := firstNonEmpty(r.Currency, r.SalaryCurrency)
	min, max := r.Min, r.Max
	if min == nil {
		min = r.SalaryFrom
	}
	if max == nil {
		max = r.SalaryTo
	}
	if !strings.EqualFold(strings.TrimSpace(currency), "IDR") {
		return ""
	}
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("Rp %g - Rp %g", *min, *max)
	case min != nil:
		return fmt.Sprintf("Rp %g", *min)
	case max != nil:
		return fmt.Sprintf("Rp %g", *max)
	default:
		return ""
	}
}

type salaryRange struct {
	Currency       string   `json:"currency"`
	SalaryCurrency string   `json:"salary_currency"`
	Min            *float64 `json:"min"`
	Max            *float64 `json:"max"`
	SalaryFrom     *float64 `json:"salary_from"`
	SalaryTo       *float64 `json:"salary_to"`
}

type greenhouseResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	Title          string `json:"title"`
	CompanyName    string `json:"company_name"`
	AbsoluteURL    string `json:"absolute_url"`
	FirstPublished string `json:"first_published"`
	UpdatedAt      string `json:"updated_at"`
	Content        string `json:"content"`
	Location       struct {
		Name string `json:"name"`
	} `json:"location"`
}

type leverJob struct {
	Text             string `json:"text"`
	Description      string `json:"description"`
	DescriptionPlain string `json:"descriptionPlain"`
	HostedURL        string `json:"hostedUrl"`
	ApplyURL         string `json:"applyUrl"`
	WorkplaceType    string `json:"workplaceType"`
	CreatedAt        int64  `json:"createdAt"`
	Categories       struct {
		Location     string   `json:"location"`
		AllLocations []string `json:"allLocations"`
		Commitment   string   `json:"commitment"`
	} `json:"categories"`
	SalaryRange salaryRange `json:"salaryRange"`
}

type workableResponse struct {
	Name string        `json:"name"`
	Jobs []workableJob `json:"jobs"`
}

type workableJob struct {
	Title           string            `json:"title"`
	URL             string            `json:"url"`
	ApplicationURL  string            `json:"application_url"`
	Shortlink       string            `json:"shortlink"`
	Location        json.RawMessage   `json:"location"`
	Locations       []json.RawMessage `json:"locations"`
	Telecommuting   bool              `json:"telecommuting"`
	Salary          salaryRange       `json:"salary"`
	Experience      string            `json:"experience"`
	EmploymentType  string            `json:"employment_type"`
	PublishedOn     string            `json:"published_on"`
	CreatedAt       string            `json:"created_at"`
	Description     string            `json:"description"`
	FullDescription string            `json:"full_description"`
}

type ashbyResponse struct {
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	Title            string `json:"title"`
	Location         string `json:"location"`
	IsListed         *bool  `json:"isListed"`
	IsRemote         bool   `json:"isRemote"`
	WorkplaceType    string `json:"workplaceType"`
	DescriptionHTML  string `json:"descriptionHtml"`
	DescriptionPlain string `json:"descriptionPlain"`
	PublishedAt      string `json:"publishedAt"`
	EmploymentType   string `json:"employmentType"`
	JobURL           string `json:"jobUrl"`
	ApplyURL         string `json:"applyUrl"`
	Compensation     struct {
		SalarySummary string `json:"scrapeableCompensationSalarySummary"`
	} `json:"compensation"`
}

func workableLocation(primary json.RawMessage, alternatives []json.RawMessage) (string, bool) {
	for _, raw := range append([]json.RawMessage{primary}, alternatives...) {
		location, telecommuting := decodeWorkableLocation(raw)
		if location != "" {
			return location, telecommuting
		}
	}
	return "", false
}

func decodeWorkableLocation(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text), detectRemote(text)
	}
	var value struct {
		LocationStr   string `json:"location_str"`
		City          string `json:"city"`
		Region        string `json:"region"`
		Country       string `json:"country"`
		Telecommuting bool   `json:"telecommuting"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	location := firstNonEmpty(value.LocationStr, strings.Join(filterNonEmpty(value.City, value.Region, value.Country), ", "))
	return location, value.Telecommuting || detectRemote(location)
}

func filterNonEmpty(values ...string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, strings.TrimSpace(value))
		}
	}
	return filtered
}
