// Package m5 contains the post-MVP seeker features: salary estimates,
// requirement tags, matching, saved filters, job state, application tracking,
// and reusable answer snippets.
package m5

import (
	"context"
	"errors"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

var (
	ErrNotFound          = errors.New("m5: not found")
	ErrConflict          = errors.New("m5: already exists")
	ErrInvalidStatus     = errors.New("m5: invalid application status")
	ErrJobNotFound       = errors.New("m5: job not found")
	ErrMailerUnavailable = errors.New("m5: mailer unavailable")
)

const (
	StatusFormFilled = "form_filled"
	StatusSubmitted  = "submitted"
	StatusViewed     = "viewed"
	StatusRejected   = "rejected"
	StatusInterview  = "interview"
)

var applicationStatuses = map[string]bool{
	StatusFormFilled: true,
	StatusSubmitted:  true,
	StatusViewed:     true,
	StatusRejected:   true,
	StatusInterview:  true,
}

type SavedFilter struct {
	ID            string          `json:"id"`
	UserID        string          `json:"-"`
	Name          string          `json:"name"`
	Query         ingest.JobQuery `json:"query"`
	CreatedAt     time.Time       `json:"created_at"`
	LastAlertedAt *time.Time      `json:"last_alerted_at"`
}

type JobState struct {
	JobKey         string    `json:"job_key"`
	Dismissed      bool      `json:"dismissed"`
	AlreadyApplied bool      `json:"already_applied"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Application struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	JobKey    string    `json:"job_key"`
	JobTitle  string    `json:"job_title"`
	Company   string    `json:"company"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Snippet struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserData struct {
	SavedFilters []SavedFilter `json:"saved_filters"`
	JobStates    []JobState    `json:"job_states"`
	Applications []Application `json:"applications"`
	Snippets     []Snippet     `json:"snippets"`
}

// Store is the persistence boundary for M5 user state. Job keys are the stable
// source-independent dedup keys already used by the feed.
type Store interface {
	DeleteUser(context.Context, string) error
	ListSavedFilterUsers(context.Context) ([]string, error)
	ListSavedFilters(context.Context, string) ([]SavedFilter, error)
	GetSavedFilter(context.Context, string, string) (SavedFilter, error)
	CreateSavedFilter(context.Context, SavedFilter) (SavedFilter, error)
	DeleteSavedFilter(context.Context, string, string) error
	TouchSavedFilter(context.Context, string, string, time.Time) error

	GetJobState(context.Context, string, string) (JobState, error)
	SetJobState(context.Context, string, JobState) (JobState, error)

	ListApplications(context.Context, string) ([]Application, error)
	GetApplication(context.Context, string, string) (Application, error)
	CreateApplication(context.Context, Application) (Application, error)
	UpdateApplication(context.Context, string, Application) (Application, error)

	ListSnippets(context.Context, string) ([]Snippet, error)
	GetSnippet(context.Context, string, string) (Snippet, error)
	CreateSnippet(context.Context, Snippet) (Snippet, error)
	UpdateSnippet(context.Context, string, Snippet) (Snippet, error)
	DeleteSnippet(context.Context, string, string) error
}

func validStatus(status string) bool { return applicationStatuses[status] }
