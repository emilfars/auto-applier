package m5

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/profile"
)

type SalaryEstimate struct {
	Min        int64
	Max        int64
	Confidence string
}

var estimateBands = map[string][2]int64{
	"intern":  {4_000_000, 6_000_000},
	"entry":   {6_000_000, 9_000_000},
	"junior":  {8_000_000, 12_000_000},
	"mid":     {12_000_000, 20_000_000},
	"senior":  {18_000_000, 30_000_000},
	"lead":    {25_000_000, 40_000_000},
	"manager": {25_000_000, 45_000_000},
}

// EstimateSalary returns a deliberately coarse IDR range only when the source
// did not state pay. It is deterministic, and callers must label the result as
// estimated rather than employer-provided.
func EstimateSalary(job ingest.Job) (SalaryEstimate, bool) {
	if job.SalaryStatedMin != nil || job.SalaryStatedMax != nil ||
		(job.SalaryCurrency != "" && !strings.EqualFold(job.SalaryCurrency, "IDR")) {
		return SalaryEstimate{}, false
	}
	level := strings.ToLower(strings.TrimSpace(job.Seniority))
	if level == "" {
		level = deriveLevel(job.Title)
	}
	band, ok := estimateBands[level]
	if !ok {
		band = [2]int64{8_000_000, 15_000_000}
	}
	factor := 1.0
	location := strings.ToLower(job.Location)
	if strings.Contains(location, "jakarta") {
		factor = 1.15
	} else if job.Remote {
		factor = 1.05
	}
	min := roundSalary(float64(band[0]) * factor)
	max := roundSalary(float64(band[1]) * factor)
	confidence := "low"
	if level != "" && (job.Location != "" || job.Remote) {
		confidence = "medium"
	}
	return SalaryEstimate{Min: min, Max: max, Confidence: confidence}, true
}

func deriveLevel(title string) string {
	value := strings.ToLower(title)
	for _, level := range []string{"manager", "lead", "senior", "mid", "junior", "entry", "intern"} {
		if containsWord(value, level) {
			return level
		}
	}
	return ""
}

func roundSalary(value float64) int64 { return int64(value/500_000+0.5) * 500_000 }

var tagAliases = map[string]string{
	"golang": "Go", "go": "Go", "postgres": "PostgreSQL", "postgresql": "PostgreSQL",
	"javascript": "JavaScript", "js": "JavaScript", "typescript": "TypeScript", "ts": "TypeScript",
	"react": "React", "node": "Node.js", "nodejs": "Node.js", "node.js": "Node.js", "python": "Python", "java": "Java",
	"kotlin": "Kotlin", "swift": "Swift", "sql": "SQL", "docker": "Docker", "kubernetes": "Kubernetes",
	"aws": "AWS", "gcp": "GCP", "azure": "Azure", "git": "Git", "excel": "Excel",
	"figma": "Figma", "graphql": "GraphQL", "rest": "REST", "redis": "Redis", "mongodb": "MongoDB",
	"communication": "communication", "leadership": "leadership", "analytics": "analytics",
}

var tagWordRe = regexp.MustCompile(`[a-z0-9][a-z0-9+#.]*`)

// ExtractRequirementTags turns free-form requirements into stable tags. Known
// tools keep their conventional casing; short unknown requirements stay useful
// as lower-case tags, while experience prose is ignored.
func ExtractRequirementTags(requirements []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	add := func(tag string) {
		if tag != "" && !seen[strings.ToLower(tag)] {
			seen[strings.ToLower(tag)] = true
			out = append(out, tag)
		}
	}
	for _, requirement := range requirements {
		text := strings.ToLower(strings.TrimSpace(requirement))
		if text == "" || strings.Contains(text, "year") || strings.Contains(text, "tahun") {
			continue
		}
		known := false
		for _, word := range tagWordRe.FindAllString(text, -1) {
			if tag, ok := tagAliases[word]; ok {
				add(tag)
				known = true
			}
		}
		words := strings.Fields(text)
		if !known && len(words) <= 3 && len(words) > 0 && len(words[0]) >= 3 && !containsDigit(text) {
			add(strings.Join(words, " "))
		}
	}
	return out
}

func containsDigit(value string) bool {
	for _, r := range value {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func containsWord(text, word string) bool {
	for _, value := range tagWordRe.FindAllString(strings.ToLower(text), -1) {
		if value == word {
			return true
		}
	}
	return false
}

// MatchScore returns the percentage of extracted job tags covered by profile
// skills. The boolean distinguishes a loaded profile from a public feed card.
func MatchScore(job ingest.Job, p profile.Profile) (int, bool) {
	var skills []string
	if err := json.Unmarshal(p.Skills, &skills); err != nil {
		return 0, true
	}
	tags := ExtractRequirementTags(job.Requirements)
	if len(tags) == 0 {
		return 0, true
	}
	matched := 0
	for _, tag := range tags {
		for _, skill := range skills {
			if strings.Contains(strings.ToLower(skill), strings.ToLower(tag)) ||
				strings.Contains(strings.ToLower(tag), strings.ToLower(skill)) {
				matched++
				break
			}
		}
	}
	return matched * 100 / len(tags), true
}

type UserStore interface {
	UserByID(context.Context, string) (auth.User, error)
}

type Service struct {
	store Store
	jobs  interface {
		ActiveJobs(context.Context) ([]ingest.Job, error)
	}
	users    UserStore
	profiles interface {
		Get(context.Context, string) (profile.Profile, error)
	}
	mailer auth.Mailer
	now    func() time.Time
}

func NewService(store Store, jobs interface {
	ActiveJobs(context.Context) ([]ingest.Job, error)
}, users UserStore,
	profiles interface {
		Get(context.Context, string) (profile.Profile, error)
	}, mailer auth.Mailer, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, jobs: jobs, users: users, profiles: profiles, mailer: mailer, now: now}
}

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /saved-filters", s.handleListFilters)
	mux.HandleFunc("POST /saved-filters", s.handleCreateFilter)
	mux.HandleFunc("DELETE /saved-filters/{id}", s.handleDeleteFilter)
	mux.HandleFunc("POST /saved-filters/{id}/notify", s.handleNotifyFilter)
	mux.HandleFunc("POST /jobs/{job_key}/dismiss", s.handleDismiss)
	mux.HandleFunc("DELETE /jobs/{job_key}/dismiss", s.handleUndismiss)
	mux.HandleFunc("POST /jobs/{job_key}/applied", s.handleApplied)
	mux.HandleFunc("GET /applications", s.handleListApplications)
	mux.HandleFunc("POST /applications", s.handleCreateApplication)
	mux.HandleFunc("PATCH /applications/{id}", s.handleUpdateApplication)
	mux.HandleFunc("POST /applications/{id}/confirm", s.handleConfirmSubmit)
	mux.HandleFunc("GET /snippets", s.handleListSnippets)
	mux.HandleFunc("POST /snippets", s.handleCreateSnippet)
	mux.HandleFunc("PATCH /snippets/{id}", s.handleUpdateSnippet)
	mux.HandleFunc("DELETE /snippets/{id}", s.handleDeleteSnippet)
	mux.HandleFunc("POST /snippets/{id}/render", s.handleRenderSnippet)
	return mux
}

func userFrom(r *http.Request) (auth.User, bool) { return auth.UserFrom(r.Context()) }

func requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	u, ok := userFrom(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
	}
	return u, ok
}

type filterRequest struct {
	Name  string          `json:"name"`
	Query ingest.JobQuery `json:"query"`
}

type filterResponse struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Query         ingest.JobQuery `json:"query"`
	CreatedAt     time.Time       `json:"created_at"`
	LastAlertedAt *time.Time      `json:"last_alerted_at"`
}

func filterCard(f SavedFilter) filterResponse {
	return filterResponse{ID: f.ID, Name: f.Name, Query: f.Query, CreatedAt: f.CreatedAt.UTC(), LastAlertedAt: f.LastAlertedAt}
}

func (s *Service) handleListFilters(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	filters, err := s.store.ListSavedFilters(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list saved filters")
		return
	}
	out := make([]filterResponse, 0, len(filters))
	for _, f := range filters {
		out = append(out, filterCard(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"filters": out})
}

func (s *Service) handleCreateFilter(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req filterRequest
	if err := decodeBody(w, r, &req, 64<<10); err != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 100 {
		writeErr(w, http.StatusBadRequest, "name and query are required")
		return
	}
	if err := ingest.ValidateJobQuery(req.Query); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	f, err := s.store.CreateSavedFilter(r.Context(), SavedFilter{UserID: u.ID, Name: req.Name, Query: req.Query})
	if errors.Is(err, ErrConflict) {
		writeErr(w, http.StatusConflict, "saved filter already exists")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create saved filter")
		return
	}
	writeJSON(w, http.StatusCreated, filterCard(f))
}

func (s *Service) handleDeleteFilter(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteSavedFilter(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "saved filter not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete saved filter")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleNotifyFilter(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	filter, err := s.store.GetSavedFilter(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "saved filter not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load saved filter")
		return
	}
	count, err := s.notifyFilter(r.Context(), u, filter)
	if errors.Is(err, ErrMailerUnavailable) {
		writeErr(w, http.StatusServiceUnavailable, "email alerts are not configured")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "send alert")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"matches": count})
}

func (s *Service) notifyFilter(ctx context.Context, user auth.User, filter SavedFilter) (int, error) {
	if s.jobs == nil || s.mailer == nil {
		return 0, ErrMailerUnavailable
	}
	jobs, err := s.jobs.ActiveJobs(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := filter.CreatedAt
	if filter.LastAlertedAt != nil {
		cutoff = *filter.LastAlertedAt
	}
	count := 0
	for _, job := range jobs {
		if job.PostedAt != nil && job.PostedAt.After(cutoff) && matches(job, filter.Query) {
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	if err := s.mailer.Send(ctx, user.Email, "New Auto Applier matches", fmt.Sprintf("%d new job(s) match your saved filter %q.", count, filter.Name)); err != nil {
		return 0, err
	}
	if err := s.store.TouchSavedFilter(ctx, user.ID, filter.ID, s.now().UTC()); err != nil {
		return 0, err
	}
	return count, nil
}

// NotifyNewMatches is used by the scheduler or a maintenance job after ingest.
func (s *Service) NotifyNewMatches(ctx context.Context, userID string) (int, error) {
	if s.users == nil {
		return 0, ErrMailerUnavailable
	}
	u, err := s.users.UserByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !u.Verified {
		return 0, nil
	}
	filters, err := s.store.ListSavedFilters(ctx, userID)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, filter := range filters {
		count, notifyErr := s.notifyFilter(ctx, u, filter)
		if notifyErr != nil {
			return total, notifyErr
		}
		total += count
	}
	return total, nil
}

// NotifyAllMatches delivers one alert pass for every user with a saved filter.
// The API process calls this from a small periodic loop when SMTP is configured.
func (s *Service) NotifyAllMatches(ctx context.Context) (int, error) {
	users, err := s.store.ListSavedFilterUsers(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, userID := range users {
		count, notifyErr := s.NotifyNewMatches(ctx, userID)
		if notifyErr != nil {
			return total, notifyErr
		}
		total += count
	}
	return total, nil
}

func (s *Service) handleDismiss(w http.ResponseWriter, r *http.Request) {
	s.updateJobState(w, r, true, false)
}
func (s *Service) handleUndismiss(w http.ResponseWriter, r *http.Request) {
	s.updateJobState(w, r, false, false)
}
func (s *Service) handleApplied(w http.ResponseWriter, r *http.Request) {
	s.updateJobState(w, r, false, true)
}

func (s *Service) updateJobState(w http.ResponseWriter, r *http.Request, dismissed, applied bool) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.PathValue("job_key"))
	if key == "" || len(key) > 128 {
		writeErr(w, http.StatusBadRequest, "invalid job key")
		return
	}
	state, err := s.store.GetJobState(r.Context(), u.ID, key)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load job state")
		return
	}
	if dismissed {
		state.Dismissed = true
	} else if applied {
		state.AlreadyApplied = true
	} else {
		state.Dismissed = false
	}
	state.JobKey = key
	saved, err := s.store.SetJobState(r.Context(), u.ID, state)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save job state")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

type applicationRequest struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

func (s *Service) handleListApplications(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	apps, err := s.store.ListApplications(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list applications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": apps})
}

func (s *Service) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req applicationRequest
	if err := decodeBody(w, r, &req, 16<<10); err != nil || strings.TrimSpace(req.JobID) == "" || len(req.JobID) > 128 {
		writeErr(w, http.StatusBadRequest, "job_id is required")
		return
	}
	status := req.Status
	if status == "" {
		status = StatusFormFilled
	}
	if status != StatusFormFilled {
		writeErr(w, http.StatusBadRequest, "new applications must start as form_filled")
		return
	}
	app, err := s.store.CreateApplication(r.Context(), Application{UserID: u.ID, JobKey: req.JobID, Status: status})
	if errors.Is(err, ErrJobNotFound) {
		writeErr(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create application")
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (s *Service) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req applicationRequest
	if err := decodeBody(w, r, &req, 8<<10); err != nil || !validStatus(req.Status) {
		writeErr(w, http.StatusBadRequest, "invalid application status")
		return
	}
	current, err := s.store.GetApplication(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "application not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load application")
		return
	}
	if current.Status != StatusFormFilled && req.Status == StatusFormFilled {
		writeErr(w, http.StatusConflict, "application status cannot move back to form_filled")
		return
	}
	updated, err := s.store.UpdateApplication(r.Context(), u.ID, Application{ID: current.ID, Status: req.Status})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update application")
		return
	}
	if req.Status == StatusSubmitted {
		if err := s.markApplied(r, u.ID, current.JobKey); err != nil {
			writeErr(w, http.StatusInternalServerError, "save applied state")
			return
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Service) handleConfirmSubmit(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	current, err := s.store.GetApplication(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "application not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load application")
		return
	}
	updated, err := s.store.UpdateApplication(r.Context(), u.ID, Application{ID: current.ID, Status: StatusSubmitted})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "confirm application")
		return
	}
	if err := s.markApplied(r, u.ID, current.JobKey); err != nil {
		writeErr(w, http.StatusInternalServerError, "save applied state")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Service) markApplied(r *http.Request, userID, jobKey string) error {
	state, err := s.store.GetJobState(r.Context(), userID, jobKey)
	if err != nil {
		return err
	}
	state.JobKey = jobKey
	state.AlreadyApplied = true
	_, err = s.store.SetJobState(r.Context(), userID, state)
	return err
}

type snippetRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (s *Service) handleListSnippets(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	snippets, err := s.store.ListSnippets(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list snippets")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snippets": snippets})
}

func validSnippet(name, body string) bool {
	return strings.TrimSpace(name) != "" && len(name) <= 100 && strings.TrimSpace(body) != "" && len(body) <= 20_000
}

func (s *Service) handleCreateSnippet(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req snippetRequest
	if err := decodeBody(w, r, &req, 24<<10); err != nil || !validSnippet(req.Name, req.Body) {
		writeErr(w, http.StatusBadRequest, "name and body are required")
		return
	}
	snippet, err := s.store.CreateSnippet(r.Context(), Snippet{UserID: u.ID, Name: req.Name, Body: req.Body})
	if errors.Is(err, ErrConflict) {
		writeErr(w, http.StatusConflict, "snippet already exists")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create snippet")
		return
	}
	writeJSON(w, http.StatusCreated, snippet)
}

func (s *Service) handleUpdateSnippet(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req snippetRequest
	if err := decodeBody(w, r, &req, 24<<10); err != nil || !validSnippet(req.Name, req.Body) {
		writeErr(w, http.StatusBadRequest, "name and body are required")
		return
	}
	snippet, err := s.store.UpdateSnippet(r.Context(), u.ID, Snippet{ID: r.PathValue("id"), Name: req.Name, Body: req.Body})
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "snippet not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		writeErr(w, http.StatusConflict, "snippet already exists")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update snippet")
		return
	}
	writeJSON(w, http.StatusOK, snippet)
}

func (s *Service) handleDeleteSnippet(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteSnippet(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "snippet not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete snippet")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type renderRequest struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Company string `json:"company"`
}

func (s *Service) handleRenderSnippet(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	snippet, err := s.store.GetSnippet(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "snippet not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load snippet")
		return
	}
	var req renderRequest
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req)
	}
	if req.Name == "" && s.profiles != nil {
		if p, profileErr := s.profiles.Get(r.Context(), u.ID); profileErr == nil {
			req.Name = p.FullName
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": substitute(snippet.Body, req)})
}

func substitute(body string, req renderRequest) string {
	replacer := strings.NewReplacer(
		"{{name}}", req.Name, "{name}", req.Name,
		"{{role}}", req.Role, "{role}", req.Role,
		"{{company}}", req.Company, "{company}", req.Company,
	)
	return replacer.Replace(body)
}

func matches(job ingest.Job, q ingest.JobQuery) bool {
	if value := strings.TrimSpace(strings.ToLower(q.Search)); value != "" &&
		!strings.Contains(strings.ToLower(job.Title+" "+job.Company), value) {
		return false
	}
	if q.PayMin != nil && (job.SalaryStatedMax == nil || *job.SalaryStatedMax < *q.PayMin) {
		return false
	}
	if q.PayMax != nil && (job.SalaryStatedMin == nil || *job.SalaryStatedMin > *q.PayMax) {
		return false
	}
	if value := strings.TrimSpace(strings.ToLower(q.Location)); value != "" && !strings.Contains(strings.ToLower(job.Location), value) {
		return false
	}
	if q.Remote != nil && job.Remote != *q.Remote {
		return false
	}
	if q.EmploymentType != "" && job.EmploymentType != q.EmploymentType {
		return false
	}
	if q.Source != "" && job.Source != q.Source {
		return false
	}
	if q.MaxYoE != nil && job.YearsExperience != nil && *job.YearsExperience > *q.MaxYoE {
		return false
	}
	if q.PostedAfter != nil && (job.PostedAt == nil || job.PostedAt.Before(*q.PostedAfter)) {
		return false
	}
	for _, wanted := range q.Skills {
		wanted = strings.TrimSpace(strings.ToLower(wanted))
		if wanted == "" {
			continue
		}
		found := false
		for _, requirement := range job.Requirements {
			if strings.Contains(strings.ToLower(requirement), wanted) {
				found = true
				break
			}
		}
		if !found {
			for _, tag := range ExtractRequirementTags(job.Requirements) {
				if strings.Contains(strings.ToLower(tag), wanted) {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeErr(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
