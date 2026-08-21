package ingest

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidJobQuery marks unsafe or contradictory feed filters.
var ErrInvalidJobQuery = errors.New("ingest: invalid job query")

// JobQuery contains the feed filters shared by the in-memory and Postgres paths.
type JobQuery struct {
	Search         string     `json:"q,omitempty"`
	PayMin         *int64     `json:"pay_min,omitempty"`
	PayMax         *int64     `json:"pay_max,omitempty"`
	Location       string     `json:"location,omitempty"`
	Remote         *bool      `json:"remote,omitempty"`
	Skills         []string   `json:"skills,omitempty"`
	MaxYoE         *int       `json:"max_yoe,omitempty"`
	EmploymentType string     `json:"employment_type,omitempty"`
	Source         string     `json:"source,omitempty"`
	PostedAfter    *time.Time `json:"posted_after,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	Offset         int        `json:"offset,omitempty"`
}

// JobPage is a filtered, ordered page of jobs.
type JobPage struct {
	Total int
	Jobs  []Job
}

// ValidateJobQuery rejects invalid pay and experience filters before querying.
func ValidateJobQuery(q JobQuery) error {
	if q.PayMin != nil && *q.PayMin < 0 {
		return fmt.Errorf("%w: pay_min must be non-negative", ErrInvalidJobQuery)
	}
	if q.PayMax != nil && *q.PayMax < 0 {
		return fmt.Errorf("%w: pay_max must be non-negative", ErrInvalidJobQuery)
	}
	if q.MaxYoE != nil && *q.MaxYoE < 0 {
		return fmt.Errorf("%w: max_yoe must be non-negative", ErrInvalidJobQuery)
	}
	if q.PayMin != nil && q.PayMax != nil && *q.PayMin > *q.PayMax {
		return fmt.Errorf("%w: pay_min must not exceed pay_max", ErrInvalidJobQuery)
	}
	return nil
}
