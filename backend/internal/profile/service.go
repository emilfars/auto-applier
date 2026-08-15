package profile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
	"github.com/google/uuid"
)

// allowedEmploymentTypes constrains the employment_type field (AC-CV-4).
var allowedEmploymentTypes = map[string]bool{
	"full_time":  true,
	"part_time":  true,
	"contract":   true,
	"internship": true,
	"freelance":  true,
	"temporary":  true,
}

const maxNoticePeriodDays = 365

// Service exposes the profile + confirm-gate HTTP API over a Repo.
type Service struct {
	repo Repo
	now  func() time.Time
	cvs  CVSource
}

// CVSource supplies an owned uploaded CV for a one-shot fill snapshot.
type CVSource interface {
	FilesByUser(context.Context, string) ([]cv.File, error)
	Content(context.Context, string, string) (cv.File, []byte, error)
}

// NewService builds a profile service.
func NewService(repo Repo, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// WithCVSource enables the authenticated profile/CV fill snapshot endpoint.
func (s *Service) WithCVSource(source CVSource) *Service {
	s.cvs = source
	return s
}

// Routes returns the profile HTTP handler. Callers must wrap it with auth
// middleware (RequireVerified) so an authenticated user is in context.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /profile", s.handleGet)
	mux.HandleFunc("GET /profile/fill", s.handleFill)
	mux.HandleFunc("PATCH /profile", s.handlePatch)
	mux.HandleFunc("POST /profile/confirm", s.handleConfirm)
	mux.HandleFunc("GET /profile/arm", s.handleArm)
	return mux
}

// load returns the user's profile, or an initialised empty one if none exists.
func (s *Service) load(r *http.Request, userID string) (Profile, error) {
	p, err := s.repo.Get(r.Context(), userID)
	if err == ErrNotFound {
		return emptyProfile(userID), nil
	}
	return p, err
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	p, err := s.load(r, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load error")
		return
	}
	writeJSON(w, http.StatusOK, toResp(p))
}

type patchReq struct {
	FullName           *string          `json:"full_name"`
	Email              *string          `json:"email"`
	Phone              *string          `json:"phone"`
	LinkedInURL        *string          `json:"linkedin_url"`
	GitHubURL          *string          `json:"github_url"`
	PortfolioURL       *string          `json:"portfolio_url"`
	Address            *string          `json:"address"`
	City               *string          `json:"city"`
	Summary            *string          `json:"summary"`
	CurrentEmployer    *string          `json:"current_employer"`
	CurrentCompany     *string          `json:"current_company"`
	CurrentTitle       *string          `json:"current_title"`
	HighestEducation   *string          `json:"highest_education"`
	Education          *json.RawMessage `json:"education"`
	WorkHistory        *json.RawMessage `json:"work_history"`
	Skills             *json.RawMessage `json:"skills"`
	ExpectedSalary     json.RawMessage  `json:"expected_salary"`
	NoticePeriodDays   json.RawMessage  `json:"notice_period_days"`
	WorkAuthorization  *string          `json:"work_authorization"`
	OpenToRelocation   *bool            `json:"open_to_relocation"`
	PreferredLocations *json.RawMessage `json:"preferred_locations"`
	EmploymentType     *string          `json:"employment_type"`
}

func (s *Service) handlePatch(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req patchReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := s.load(r, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load error")
		return
	}

	if msg, ok := applyPatch(&p, req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	// Editing invalidates a prior confirmation: the user must re-review before
	// the fill flow can be armed again (confirm-before-apply gate).
	p.Confirmed = false
	p.ConfirmedAt = nil

	saved, err := s.repo.Save(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, toResp(saved))
}

// applyPatch mutates p from req, returning (errorMessage, false) on validation
// failure.
func applyPatch(p *Profile, req patchReq) (string, bool) {
	if req.FullName != nil {
		p.FullName = *req.FullName
	}
	if req.Email != nil {
		if *req.Email != "" && !validEmail(*req.Email) {
			return "email must be valid", false
		}
		p.Email = *req.Email
	}
	if req.Phone != nil {
		p.Phone = *req.Phone
	}
	for _, f := range []struct {
		src  *string
		dst  *string
		name string
	}{
		{req.LinkedInURL, &p.LinkedInURL, "linkedin_url"},
		{req.GitHubURL, &p.GitHubURL, "github_url"},
		{req.PortfolioURL, &p.PortfolioURL, "portfolio_url"},
	} {
		if f.src == nil {
			continue
		}
		value := strings.TrimSpace(*f.src)
		if value != "" && !validProfileURL(value) {
			return f.name + " must be a valid URL", false
		}
		*f.dst = value
	}
	currentEmployer := req.CurrentEmployer
	if currentEmployer == nil {
		currentEmployer = req.CurrentCompany
	}
	for _, f := range []struct {
		src *string
		dst *string
	}{
		{req.Address, &p.Address},
		{req.City, &p.City},
		{req.Summary, &p.Summary},
		{currentEmployer, &p.CurrentEmployer},
		{req.CurrentTitle, &p.CurrentTitle},
		{req.HighestEducation, &p.HighestEducation},
	} {
		if f.src != nil {
			*f.dst = strings.TrimSpace(*f.src)
		}
	}
	if req.WorkAuthorization != nil {
		p.WorkAuthorization = *req.WorkAuthorization
	}
	if req.OpenToRelocation != nil {
		p.OpenToRelocation = *req.OpenToRelocation
	}
	for _, f := range []struct {
		src      *json.RawMessage
		dst      *json.RawMessage
		name     string
		validate func(json.RawMessage) bool
	}{
		{req.Education, &p.Education, "education", validArray[cv.EducationEntry]},
		{req.WorkHistory, &p.WorkHistory, "work_history", validArray[cv.WorkEntry]},
		{req.Skills, &p.Skills, "skills", validArray[string]},
		{req.PreferredLocations, &p.PreferredLocations, "preferred_locations", validArray[string]},
	} {
		if f.src == nil {
			continue
		}
		if !f.validate(*f.src) {
			return f.name + " has an invalid array shape", false
		}
		*f.dst = append(json.RawMessage(nil), *f.src...)
	}
	if len(req.ExpectedSalary) > 0 {
		if bytes.Equal(req.ExpectedSalary, []byte("null")) {
			p.ExpectedSalary = nil
		} else {
			var value int64
			if err := json.Unmarshal(req.ExpectedSalary, &value); err != nil || value < 0 {
				return "expected_salary must be null or a non-negative integer", false
			}
			p.ExpectedSalary = &value
		}
	}
	if len(req.NoticePeriodDays) > 0 {
		if bytes.Equal(req.NoticePeriodDays, []byte("null")) {
			p.NoticePeriodDays = nil
		} else {
			var value int
			if err := json.Unmarshal(req.NoticePeriodDays, &value); err != nil ||
				value < 0 || value > maxNoticePeriodDays {
				return "notice_period_days must be null or between 0 and 365", false
			}
			p.NoticePeriodDays = &value
		}
	}
	if req.EmploymentType != nil {
		if *req.EmploymentType != "" && !allowedEmploymentTypes[*req.EmploymentType] {
			return "employment_type is not a recognised value", false
		}
		p.EmploymentType = *req.EmploymentType
	}
	return "", true
}

func validArray[T any](raw json.RawMessage) bool {
	var values []T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&values); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func validProfileURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil &&
		parsed.IsAbs() &&
		(strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")) &&
		parsed.Hostname() != "" &&
		parsed.User == nil
}

func profileComplete(p Profile) bool {
	return strings.TrimSpace(p.FullName) != "" &&
		validEmail(p.Email) &&
		strings.TrimSpace(p.Phone) != "" &&
		strings.TrimSpace(p.WorkAuthorization) != "" &&
		allowedEmploymentTypes[p.EmploymentType] &&
		hasEducation(p.Education) &&
		hasNonBlankString(p.Skills) &&
		hasNonBlankString(p.PreferredLocations)
}

func hasEducation(raw json.RawMessage) bool {
	var values []cv.EducationEntry
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value.Institution) != "" {
			return true
		}
	}
	return false
}

func hasNonBlankString(raw json.RawMessage) bool {
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func (s *Service) handleConfirm(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	p, err := s.load(r, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load error")
		return
	}
	if !profileComplete(p) {
		writeErr(w, http.StatusBadRequest, "profile is incomplete")
		return
	}
	now := s.now().UTC()
	p.Confirmed = true
	p.ConfirmedAt = &now
	saved, err := s.repo.Save(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save error")
		return
	}
	writeJSON(w, http.StatusOK, toResp(saved))
}

// handleArm reports whether the fill flow may be armed. It returns 403 while the
// profile is unconfirmed — the fill flow must refuse to arm in that case.
func (s *Service) handleArm(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	p, err := s.load(r, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load error")
		return
	}
	if !p.CanArm() {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"can_arm": false,
			"reason":  "profile not confirmed",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"can_arm": true})
}

type fillCVResp struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	BytesBase64 string `json:"bytes_base64"`
}

// handleFill returns only the confirmed, caller-owned data needed for one
// Open & Fill arm. The response is never persisted by the backend.
func (s *Service) handleFill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	cvID := r.URL.Query().Get("cv_id")
	if cvID != "" {
		if _, err := uuid.Parse(cvID); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid cv_id")
			return
		}
	}
	p, err := s.load(r, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load error")
		return
	}
	if !p.CanArm() {
		writeErr(w, http.StatusForbidden, "profile not confirmed")
		return
	}

	out := map[string]any{
		"profile": canonicalFillProfile(p),
		"cv":      nil,
	}
	if s.cvs == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}

	if cvID == "" {
		files, err := s.cvs.FilesByUser(r.Context(), u.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "list cv files error")
			return
		}
		if len(files) > 0 {
			cvID = files[0].ID
		}
	}
	if cvID != "" {
		file, data, err := s.cvs.Content(r.Context(), u.ID, cvID)
		if errors.Is(err, cv.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "cv not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "read cv error")
			return
		}
		out["cv"] = fillCVResp{
			ID: file.ID, Filename: file.Filename, ContentType: file.ContentType,
			BytesBase64: base64.StdEncoding.EncodeToString(data),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func canonicalFillProfile(p Profile) map[string]any {
	out := map[string]any{"confirmed": p.Confirmed}
	if v := strings.TrimSpace(p.FullName); v != "" {
		out["full_name"] = v
		parts := strings.Fields(v)
		out["first_name"] = parts[0]
		if len(parts) > 1 {
			out["last_name"] = parts[len(parts)-1]
		}
	}
	if v := strings.TrimSpace(p.Email); v != "" {
		out["email"] = v
	}
	if v := strings.TrimSpace(p.Phone); v != "" {
		out["phone"] = v
	}
	for _, field := range []struct {
		key   string
		value string
	}{
		{"linkedin_url", p.LinkedInURL},
		{"github_url", p.GitHubURL},
		{"portfolio_url", p.PortfolioURL},
		{"address", p.Address},
		{"city", p.City},
		{"summary", p.Summary},
		{"current_company", p.CurrentEmployer},
		{"current_title", p.CurrentTitle},
		{"highest_education", p.HighestEducation},
	} {
		if value := strings.TrimSpace(field.value); value != "" {
			out[field.key] = value
		}
	}
	if p.ExpectedSalary != nil {
		out["expected_salary"] = strconv.FormatInt(*p.ExpectedSalary, 10)
	}
	if p.NoticePeriodDays != nil {
		out["notice_period"] = strconv.Itoa(*p.NoticePeriodDays)
	}

	var education []cv.EducationEntry
	if json.Unmarshal(p.Education, &education) == nil && len(education) > 0 {
		entry := education[len(education)-1]
		if strings.TrimSpace(entry.Institution) != "" {
			out["university"] = entry.Institution
		}
		if strings.TrimSpace(entry.Field) != "" {
			out["major"] = entry.Field
		}
		if strings.TrimSpace(entry.EndYear) != "" {
			out["graduation_year"] = entry.EndYear
		}
	}

	return out
}

func isJSONArray(raw json.RawMessage) bool {
	var arr []json.RawMessage
	return json.Unmarshal(raw, &arr) == nil
}

type profileResp struct {
	FullName           string          `json:"full_name"`
	Email              string          `json:"email"`
	Phone              string          `json:"phone"`
	LinkedInURL        string          `json:"linkedin_url"`
	GitHubURL          string          `json:"github_url"`
	PortfolioURL       string          `json:"portfolio_url"`
	Address            string          `json:"address"`
	City               string          `json:"city"`
	Summary            string          `json:"summary"`
	CurrentEmployer    string          `json:"current_employer"`
	CurrentCompany     string          `json:"current_company"`
	CurrentTitle       string          `json:"current_title"`
	HighestEducation   string          `json:"highest_education"`
	Education          json.RawMessage `json:"education"`
	WorkHistory        json.RawMessage `json:"work_history"`
	Skills             json.RawMessage `json:"skills"`
	ExpectedSalary     *int64          `json:"expected_salary"`
	NoticePeriodDays   *int            `json:"notice_period_days"`
	WorkAuthorization  string          `json:"work_authorization"`
	OpenToRelocation   bool            `json:"open_to_relocation"`
	PreferredLocations json.RawMessage `json:"preferred_locations"`
	EmploymentType     string          `json:"employment_type"`
	Confirmed          bool            `json:"confirmed"`
	ConfirmedAt        *string         `json:"confirmed_at"`
}

func toResp(p Profile) profileResp {
	nonNil := func(raw json.RawMessage) json.RawMessage {
		if len(raw) == 0 {
			return json.RawMessage("[]")
		}
		return raw
	}
	var confirmedAt *string
	if p.ConfirmedAt != nil {
		s := p.ConfirmedAt.Format(time.RFC3339)
		confirmedAt = &s
	}
	return profileResp{
		FullName:           p.FullName,
		Email:              p.Email,
		Phone:              p.Phone,
		LinkedInURL:        p.LinkedInURL,
		GitHubURL:          p.GitHubURL,
		PortfolioURL:       p.PortfolioURL,
		Address:            p.Address,
		City:               p.City,
		Summary:            p.Summary,
		CurrentEmployer:    p.CurrentEmployer,
		CurrentCompany:     p.CurrentEmployer,
		CurrentTitle:       p.CurrentTitle,
		HighestEducation:   p.HighestEducation,
		Education:          nonNil(p.Education),
		WorkHistory:        nonNil(p.WorkHistory),
		Skills:             nonNil(p.Skills),
		ExpectedSalary:     p.ExpectedSalary,
		NoticePeriodDays:   p.NoticePeriodDays,
		WorkAuthorization:  p.WorkAuthorization,
		OpenToRelocation:   p.OpenToRelocation,
		PreferredLocations: nonNil(p.PreferredLocations),
		EmploymentType:     p.EmploymentType,
		Confirmed:          p.Confirmed,
		ConfirmedAt:        confirmedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
