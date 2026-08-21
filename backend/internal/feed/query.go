// Package feed serves the browsable, filterable job feed. Stale listings are
// excluded; M5 salary estimates remain separate from stated compensation.
package feed

import (
	"sort"
	"strings"

	"github.com/auto-applier/backend/internal/ingest"
)

// Query is a feed request: free-text search (FEED-3) plus filters (FEED-2).
// Zero-valued fields are ignored.
type Query = ingest.JobQuery

// Index is the in-memory query path used by tests and local development.
type Index struct {
	jobs []ingest.Job
}

// NewIndex builds an index over the given (already active, non-stale) jobs.
func NewIndex(jobs []ingest.Job) *Index {
	cp := make([]ingest.Job, len(jobs))
	copy(cp, jobs)
	return &Index{jobs: cp}
}

const defaultLimit = 20

// Result is a page of feed cards plus the total number of matches.
type Result struct {
	Total int
	Jobs  []ingest.Job
}

// Search applies the query filters and returns a page ordered by posting date
// (newest first), then title and dedup key for stability.
func (ix *Index) Search(q Query) Result {
	var matched []ingest.Job
	for _, j := range ix.jobs {
		if matches(j, q) {
			matched = append(matched, j)
		}
	}
	sort.Slice(matched, func(i, k int) bool {
		ti, tk := matched[i].PostedAt, matched[k].PostedAt
		switch {
		case ti != nil && tk != nil && !ti.Equal(*tk):
			return ti.After(*tk)
		case (ti == nil) != (tk == nil):
			return ti != nil // dated listings sort ahead of undated
		case matched[i].Title != matched[k].Title:
			return matched[i].Title < matched[k].Title
		default:
			return matched[i].DedupKey < matched[k].DedupKey
		}
	})

	total := len(matched)
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	start := q.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return Result{Total: total, Jobs: matched[start:end]}
}

func matches(j ingest.Job, q Query) bool {
	if s := strings.TrimSpace(strings.ToLower(q.Search)); s != "" {
		hay := strings.ToLower(j.Title + " " + j.Company)
		if !strings.Contains(hay, s) {
			return false
		}
	}
	// Pay overlap: the job's stated range must intersect [PayMin, PayMax].
	if q.PayMin != nil {
		if j.SalaryStatedMax == nil || *j.SalaryStatedMax < *q.PayMin {
			return false
		}
	}
	if q.PayMax != nil {
		if j.SalaryStatedMin == nil || *j.SalaryStatedMin > *q.PayMax {
			return false
		}
	}
	if loc := strings.TrimSpace(strings.ToLower(q.Location)); loc != "" {
		if !strings.Contains(strings.ToLower(j.Location), loc) {
			return false
		}
	}
	if q.Remote != nil && j.Remote != *q.Remote {
		return false
	}
	if q.EmploymentType != "" && j.EmploymentType != q.EmploymentType {
		return false
	}
	if q.Source != "" && j.Source != q.Source {
		return false
	}
	if q.MaxYoE != nil {
		// Listings with no stated YoE are treated as accessible (0 required).
		if j.YearsExperience != nil && *j.YearsExperience > *q.MaxYoE {
			return false
		}
	}
	if q.PostedAfter != nil {
		if j.PostedAt == nil || j.PostedAt.Before(*q.PostedAfter) {
			return false
		}
	}
	if len(q.Skills) > 0 {
		reqs := make([]string, len(j.Requirements))
		for i, r := range j.Requirements {
			reqs[i] = strings.ToLower(r)
		}
		for _, want := range q.Skills {
			want = strings.ToLower(strings.TrimSpace(want))
			if want == "" {
				continue
			}
			found := false
			for _, r := range reqs {
				if strings.Contains(r, want) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
