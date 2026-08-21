package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/m5"
	"github.com/auto-applier/backend/internal/profile"
)

// Provider supplies the active (non-stale) jobs the feed serves.
type Provider interface {
	ActiveJobs(ctx context.Context) ([]ingest.Job, error)
}

type queryProvider interface {
	SearchFeed(ctx context.Context, q ingest.JobQuery) (ingest.JobPage, error)
}

type personalizedQueryProvider interface {
	SearchFeedForUser(context.Context, ingest.JobQuery, string) (ingest.JobPage, error)
}

type profileProvider interface {
	Get(context.Context, string) (profile.Profile, error)
}

type stateProvider interface {
	GetJobState(context.Context, string, string) (m5.JobState, error)
}

// Service exposes the feed HTTP API.
type Service struct {
	provider Provider
	profiles profileProvider
	states   stateProvider
}

// NewService builds a feed service over a job provider.
func NewService(p Provider) *Service { return &Service{provider: p} }

// WithPersonalization adds optional per-user match scores and job state. The
// public feed remains unchanged for anonymous visitors.
func (s *Service) WithPersonalization(profiles profileProvider, states stateProvider) *Service {
	s.profiles = profiles
	s.states = states
	return s
}

// Routes returns the feed HTTP handler. The feed is public (browsable before
// signup) so it needs no auth middleware.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /feed", s.handleFeed)
	return mux
}

// salaryCard keeps stated and estimated pay separate so estimates cannot be
// mistaken for employer-provided compensation.
type salaryCard struct {
	StatedMin    *int64 `json:"stated_min"`
	StatedMax    *int64 `json:"stated_max"`
	EstimatedMin *int64 `json:"estimated_min,omitempty"`
	EstimatedMax *int64 `json:"estimated_max,omitempty"`
	Currency     string `json:"currency"`
	Label        string `json:"label"`
	Stated       bool   `json:"stated"`
	Estimated    bool   `json:"estimated"`
}

type card struct {
	DedupKey        string     `json:"dedup_key"`
	Source          string     `json:"source"`
	SourceURL       string     `json:"source_url"`
	Title           string     `json:"title"`
	Company         string     `json:"company"`
	Location        string     `json:"location"`
	Remote          bool       `json:"remote"`
	Salary          salaryCard `json:"salary"`
	Seniority       string     `json:"seniority"`
	EmploymentType  string     `json:"employment_type"`
	YearsExperience *int       `json:"years_experience"`
	Requirements    []string   `json:"requirements"`
	RequirementTags []string   `json:"requirement_tags"`
	MatchScore      *int       `json:"match_score"`
	AlreadyApplied  bool       `json:"already_applied"`
	PostedAt        *string    `json:"posted_at"`
}

type feedResp struct {
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Jobs   []card `json:"jobs"`
}

func (s *Service) handleFeed(w http.ResponseWriter, r *http.Request) {
	q, err := parseQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	q = normalizeQuery(q)

	user, signedIn := auth.UserFrom(r.Context())
	personalized := signedIn && s.states != nil
	states := map[string]m5.JobState{}
	var res Result
	if personalized {
		if provider, ok := s.provider.(personalizedQueryProvider); ok {
			page, queryErr := provider.SearchFeedForUser(r.Context(), q, user.ID)
			if queryErr != nil {
				writeErr(w, http.StatusInternalServerError, "feed unavailable")
				return
			}
			res = Result{Total: page.Total, Jobs: page.Jobs}
		} else {
			jobs, activeErr := s.provider.ActiveJobs(r.Context())
			if activeErr != nil {
				writeErr(w, http.StatusInternalServerError, "feed unavailable")
				return
			}
			visible := make([]ingest.Job, 0, len(jobs))
			for _, job := range jobs {
				state, stateErr := s.states.GetJobState(r.Context(), user.ID, job.DedupKey)
				if stateErr != nil {
					writeErr(w, http.StatusInternalServerError, "feed unavailable")
					return
				}
				states[job.DedupKey] = state
				if !state.Dismissed {
					visible = append(visible, job)
				}
			}
			res = NewIndex(visible).Search(q)
		}
	} else if provider, ok := s.provider.(queryProvider); ok {
		page, queryErr := provider.SearchFeed(r.Context(), q)
		if queryErr != nil {
			writeErr(w, http.StatusInternalServerError, "feed unavailable")
			return
		}
		res = Result{Total: page.Total, Jobs: page.Jobs}
	} else {
		jobs, activeErr := s.provider.ActiveJobs(r.Context())
		if activeErr != nil {
			writeErr(w, http.StatusInternalServerError, "feed unavailable")
			return
		}
		res = NewIndex(jobs).Search(q)
	}

	var matchingProfile profile.Profile
	hasProfile := personalized && s.profiles != nil
	if hasProfile {
		var profileErr error
		matchingProfile, profileErr = s.profiles.Get(r.Context(), user.ID)
		hasProfile = profileErr == nil
	}
	cards := make([]card, 0, len(res.Jobs))
	for _, j := range res.Jobs {
		item := toCard(j)
		if personalized {
			state, ok := states[j.DedupKey]
			if !ok {
				var stateErr error
				state, stateErr = s.states.GetJobState(r.Context(), user.ID, j.DedupKey)
				if stateErr != nil {
					writeErr(w, http.StatusInternalServerError, "feed unavailable")
					return
				}
			}
			item.AlreadyApplied = state.AlreadyApplied
		}
		if hasProfile {
			if score, ok := m5.MatchScore(j, matchingProfile); ok {
				item.MatchScore = &score
			}
		}
		cards = append(cards, item)
	}
	writeJSON(w, http.StatusOK, feedResp{Total: res.Total, Limit: q.Limit, Offset: q.Offset, Jobs: cards})
}

func normalizeQuery(q Query) Query {
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return q
}

func parseQuery(r *http.Request) (Query, error) {
	v := r.URL.Query()
	q := Query{
		Search:         v.Get("q"),
		Location:       v.Get("location"),
		EmploymentType: v.Get("employment_type"),
		Source:         v.Get("source"),
	}
	if s := v.Get("skills"); s != "" {
		q.Skills = strings.Split(s, ",")
	}
	var err error
	if q.PayMin, err = optInt64(v.Get("pay_min")); err != nil {
		return q, err
	}
	if q.PayMax, err = optInt64(v.Get("pay_max")); err != nil {
		return q, err
	}
	if q.MaxYoE, err = optInt(v.Get("max_yoe")); err != nil {
		return q, err
	}
	if rem := v.Get("remote"); rem != "" {
		b, berr := strconv.ParseBool(rem)
		if berr != nil {
			return q, errBad("remote must be true or false")
		}
		q.Remote = &b
	}
	if pa := v.Get("posted_after"); pa != "" {
		t, terr := time.Parse(time.RFC3339, pa)
		if terr != nil {
			return q, errBad("posted_after must be RFC3339")
		}
		q.PostedAfter = &t
	}
	if q.Limit, err = intDefault(v.Get("limit"), 0); err != nil {
		return q, err
	}
	if q.Offset, err = intDefault(v.Get("offset"), 0); err != nil {
		return q, err
	}
	if err := ingest.ValidateJobQuery(q); err != nil {
		return q, errBad(err.Error())
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	return q, nil
}

func toCard(j ingest.Job) card {
	var posted *string
	if j.PostedAt != nil {
		s := j.PostedAt.UTC().Format(time.RFC3339)
		posted = &s
	}
	reqs := j.Requirements
	if reqs == nil {
		reqs = []string{}
	}
	tags := m5.ExtractRequirementTags(reqs)
	if tags == nil {
		tags = []string{}
	}
	stated := j.SalaryStatedMin != nil || j.SalaryStatedMax != nil
	salary := salaryCard{
		StatedMin: j.SalaryStatedMin,
		StatedMax: j.SalaryStatedMax,
		Currency:  j.SalaryCurrency,
		Label:     salaryLabel(j),
		Stated:    stated,
	}
	if !stated {
		if estimate, ok := m5.EstimateSalary(j); ok {
			salary.EstimatedMin = &estimate.Min
			salary.EstimatedMax = &estimate.Max
			salary.Estimated = true
			salary.Label = estimatedSalaryLabel(estimate)
		}
	}
	return card{
		DedupKey:        j.DedupKey,
		Source:          j.Source,
		SourceURL:       j.SourceURL,
		Title:           j.Title,
		Company:         j.Company,
		Location:        j.Location,
		Remote:          j.Remote,
		Salary:          salary,
		Seniority:       j.Seniority,
		EmploymentType:  j.EmploymentType,
		YearsExperience: j.YearsExperience,
		Requirements:    reqs,
		RequirementTags: tags,
		PostedAt:        posted,
	}
}

func estimatedSalaryLabel(estimate m5.SalaryEstimate) string {
	return "~Rp " + grouped(estimate.Min) + " - Rp " + grouped(estimate.Max) + " (estimated)"
}

// salaryLabel formats stated pay for display. It returns an empty string when no
// pay is stated — the UI shows a neutral "Not disclosed", never an estimate.
func salaryLabel(j ingest.Job) string {
	cur := j.SalaryCurrency
	if cur == "" {
		cur = "IDR"
	}
	sym := cur
	if cur == "IDR" {
		sym = "Rp"
	}
	switch {
	case j.SalaryStatedMin != nil && j.SalaryStatedMax != nil && *j.SalaryStatedMin != *j.SalaryStatedMax:
		return sym + " " + grouped(*j.SalaryStatedMin) + " - " + sym + " " + grouped(*j.SalaryStatedMax)
	case j.SalaryStatedMin != nil:
		return sym + " " + grouped(*j.SalaryStatedMin)
	case j.SalaryStatedMax != nil:
		return sym + " " + grouped(*j.SalaryStatedMax)
	default:
		return ""
	}
}

// grouped formats an integer with dot thousands separators (Indonesian style).
func grouped(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

type badRequest struct{ msg string }

func (e badRequest) Error() string { return e.msg }
func errBad(msg string) error      { return badRequest{msg} }

func optInt64(s string) (*int64, error) {
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, errBad("invalid integer: " + s)
	}
	return &n, nil
}

func optInt(s string) (*int, error) {
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, errBad("invalid integer: " + s)
	}
	return &n, nil
}

func intDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def, errBad("invalid integer: " + s)
	}
	return n, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
