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
	Search         string
	PayMin         *int64
	PayMax         *int64
	Location       string
	Remote         *bool
	Skills         []string
	MaxYoE         *int
	EmploymentType string
	Source         string
	PostedAfter    *time.Time
	Limit          int
	Offset         int
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
