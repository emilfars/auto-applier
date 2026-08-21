// Package account implements user-facing data-rights endpoints required by
// UU PDP No. 27/2022 (AC-AUTH-5): export all of a user's data, and delete the
// account together with every piece of PII it owns.
//
// It is a thin coordinator over the auth, profile, and cv domains, depending on
// narrow capability interfaces so it stays decoupled and unit-testable with
// fakes. Per the Prime Directive it never submits anything; it only reads and
// erases the signed-in user's own data.
package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
	"github.com/auto-applier/backend/internal/profile"
)

// UserStore reads and erases the account record.
type UserStore interface {
	UserByID(ctx context.Context, id string) (auth.User, error)
	DeleteUser(ctx context.Context, id string) error
}

// ProfileStore reads and erases a user's structured profile PII.
type ProfileStore interface {
	Get(ctx context.Context, userID string) (profile.Profile, error)
	Delete(ctx context.Context, userID string) error
}

// CVStore reads a user's CV metadata and erases their CV data (objects + rows).
type CVStore interface {
	FilesByUser(ctx context.Context, userID string) ([]cv.File, error)
	DeleteUserFiles(ctx context.Context, userID string) error
}

type AdditionalStore interface {
	DeleteUser(context.Context, string) error
}

type AdditionalExporter interface {
	ExportUser(context.Context, string) (any, error)
}

// Service exposes the export + delete endpoints.
type Service struct {
	users      UserStore
	profiles   ProfileStore
	cvs        CVStore
	additional AdditionalStore
	exporter   AdditionalExporter
}

// NewService builds the account service from the three domain stores.
func NewService(users UserStore, profiles ProfileStore, cvs CVStore) *Service {
	return &Service{users: users, profiles: profiles, cvs: cvs}
}

func (s *Service) WithAdditionalStore(store AdditionalStore) *Service {
	s.additional = store
	if exporter, ok := store.(AdditionalExporter); ok {
		s.exporter = exporter
	}
	return s
}

// Routes returns the account HTTP handler. Callers must wrap it with auth
// middleware (RequireVerified) so an authenticated user is in context.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /account/export", s.handleExport)
	mux.HandleFunc("DELETE /account", s.handleDelete)
	return mux
}

// --- export ---

type accountExport struct {
	Email     string     `json:"email"`
	Verified  bool       `json:"verified"`
	CreatedAt time.Time  `json:"created_at"`
	ConsentAt *time.Time `json:"consent_at"`
}

type profileExport struct {
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
	ConfirmedAt        *time.Time      `json:"confirmed_at"`
}

type cvFileExport struct {
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

type exportResponse struct {
	Account    accountExport  `json:"account"`
	Profile    *profileExport `json:"profile"`
	CVFiles    []cvFileExport `json:"cv_files"`
	Additional any            `json:"additional,omitempty"`
	Exported   time.Time      `json:"exported_at"`
}

func (s *Service) handleExport(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	ctx := r.Context()

	// Re-read the account for authoritative timestamps.
	acct, err := s.users.UserByID(ctx, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load account")
		return
	}
	out := exportResponse{
		Account:  toAccountExport(acct),
		CVFiles:  []cvFileExport{},
		Exported: time.Now().UTC(),
	}

	// Profile is optional — a user may not have built one yet.
	if p, err := s.profiles.Get(ctx, u.ID); err == nil {
		pe := toProfileExport(p)
		out.Profile = &pe
	} else if !errors.Is(err, profile.ErrNotFound) {
		writeErr(w, http.StatusInternalServerError, "load profile")
		return
	}

	files, err := s.cvs.FilesByUser(ctx, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load cv files")
		return
	}
	for _, f := range files {
		out.CVFiles = append(out.CVFiles, cvFileExport{
			Filename:    f.Filename,
			ContentType: f.ContentType,
			SizeBytes:   f.SizeBytes,
			CreatedAt:   f.CreatedAt.UTC(),
		})
	}
	if s.exporter != nil {
		additional, exportErr := s.exporter.ExportUser(ctx, u.ID)
		if exportErr != nil {
			writeErr(w, http.StatusInternalServerError, "load additional data")
			return
		}
		out.Additional = additional
	}

	writeJSON(w, http.StatusOK, out)
}

// --- delete ---

// handleDelete erases the user's CV data (objects + metadata), profile, and
// account, in that order: CV objects are removed while their metadata still
// exists, and the account row is deleted last so nothing is orphaned.
func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	ctx := r.Context()

	if err := s.cvs.DeleteUserFiles(ctx, u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete cv data")
		return
	}
	if err := s.profiles.Delete(ctx, u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete profile")
		return
	}
	if s.additional != nil {
		if err := s.additional.DeleteUser(ctx, u.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "delete account data")
			return
		}
	}
	if err := s.users.DeleteUser(ctx, u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- mapping + helpers ---

func toAccountExport(u auth.User) accountExport {
	e := accountExport{
		Email:     u.Email,
		Verified:  u.Verified,
		CreatedAt: u.CreatedAt.UTC(),
	}
	if !u.ConsentAt.IsZero() {
		c := u.ConsentAt.UTC()
		e.ConsentAt = &c
	}
	return e
}

func toProfileExport(p profile.Profile) profileExport {
	return profileExport{
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
		CurrentTitle:       p.CurrentTitle,
		HighestEducation:   p.HighestEducation,
		Education:          p.Education,
		WorkHistory:        p.WorkHistory,
		Skills:             p.Skills,
		ExpectedSalary:     p.ExpectedSalary,
		NoticePeriodDays:   p.NoticePeriodDays,
		WorkAuthorization:  p.WorkAuthorization,
		OpenToRelocation:   p.OpenToRelocation,
		PreferredLocations: p.PreferredLocations,
		EmploymentType:     p.EmploymentType,
		Confirmed:          p.Confirmed,
		ConfirmedAt:        p.ConfirmedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
