// Package ingest contains the job ingestion domain: source definitions (Tier 1
// and Tier 2 only — login-walled Tier 3 sources are excluded by policy), the
// normalized Job model, normalization, cross-source deduplication, and
// staleness rules. Scrapers feed RawJob values in; the feed reads normalized
// Job values out.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Tier classifies a source per the locked sourcing policy.
//
//	Tier 1 — official APIs / partner feeds.
//	Tier 2 — public job boards with no login wall.
//	Tier 3 — login-walled sources (e.g. LinkedIn). FORBIDDEN — never ingested.
type Tier int

const (
	Tier1 Tier = 1
	Tier2 Tier = 2
	Tier3 Tier = 3
)

// ErrTierExcluded is returned when registering a source whose tier is not an
// allowed (Tier 1 or Tier 2) source. This enforces the locked decision at the
// code boundary so a Tier 3 scraper can never be wired into ingestion.
var ErrTierExcluded = errors.New("ingest: source tier not allowed (Tier 1/2 only)")

// RawJob is a single listing as scraped, before normalization. Fields are raw
// strings; SalaryText is parsed by the normalizer.
type RawJob struct {
	Source         string
	SourceURL      string
	Title          string
	Company        string
	Location       string
	Remote         bool
	SalaryText     string
	Seniority      string
	EmploymentType string
	Requirements   []string
	PostedAt       *time.Time
}

// Job is a normalized listing ready for the feed. Salary is stated-only at MVP
// (no estimated fields) and defaults to IDR.
type Job struct {
	Source          string
	SourceURL       string
	DedupKey        string
	Title           string
	Company         string
	Location        string
	Remote          bool
	SalaryStatedMin *int64
	SalaryStatedMax *int64
	SalaryCurrency  string
	Seniority       string
	EmploymentType  string
	YearsExperience *int
	Requirements    []string
	PostedAt        *time.Time
}

// Source produces raw listings. Implementations must be Tier 1 or Tier 2.
type Source interface {
	ID() string
	Tier() Tier
	Fetch(ctx context.Context) ([]RawJob, error)
}

// Registry holds the enabled sources. Register refuses any source that is not
// Tier 1 or Tier 2, keeping login-walled sources out of ingestion.
type Registry struct {
	sources map[string]Source
	order   []string
}

// NewRegistry returns an empty source registry.
func NewRegistry() *Registry {
	return &Registry{sources: map[string]Source{}}
}

// Register adds a source. It returns ErrTierExcluded for a Tier 3 (or unknown)
// source, and an error on duplicate/empty ids.
func (r *Registry) Register(s Source) error {
	if s.Tier() != Tier1 && s.Tier() != Tier2 {
		return fmt.Errorf("%w: %q is tier %d", ErrTierExcluded, s.ID(), s.Tier())
	}
	id := s.ID()
	if id == "" {
		return errors.New("ingest: source id is empty")
	}
	if _, ok := r.sources[id]; ok {
		return fmt.Errorf("ingest: source %q already registered", id)
	}
	r.sources[id] = s
	r.order = append(r.order, id)
	return nil
}

// Sources returns the registered sources in registration order.
func (r *Registry) Sources() []Source {
	out := make([]Source, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.sources[id])
	}
	return out
}
