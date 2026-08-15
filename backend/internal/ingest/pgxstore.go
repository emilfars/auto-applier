package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxDB is the subset of pgx the job store needs. Both *pgxpool.Pool and a
// single connection satisfy it.
type PgxDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxStore is the production Postgres-backed job store. It implements the
// ingest.JobStore (Upsert) contract used by the Runner/seed and the feed's
// Provider (ActiveJobs) contract used by the feed API — so jobs persist across
// restarts instead of living only in memory.
type PgxStore struct {
	db PgxDB
}

// NewPgxStore wraps a pgx pool/connection as a job store.
func NewPgxStore(db PgxDB) *PgxStore { return &PgxStore{db: db} }

const jobColumns = `source, source_url, dedup_key, title, company, location, remote,
	salary_stated_min, salary_stated_max, salary_currency, seniority, employment_type,
	years_experience, requirements, posted_at, synthetic`

// Upsert inserts or updates a job keyed by dedup_key (SCR-3). Re-seeing a
// listing refreshes updated_at (last-seen) and clears any stale flag (SCR-4).
// It returns whether the row was newly created (vs. an update).
func (s *PgxStore) Upsert(ctx context.Context, j Job) (bool, error) {
	if err := ValidateSourceURL(j.SourceURL); err != nil {
		return false, fmt.Errorf("validate job source url: %w", err)
	}
	currency := j.SalaryCurrency
	if currency == "" {
		currency = "IDR"
	}
	reqs, err := json.Marshal(j.Requirements)
	if err != nil {
		return false, fmt.Errorf("marshal requirements: %w", err)
	}
	if len(j.Requirements) == 0 {
		reqs = []byte("[]")
	}

	var inserted bool
	err = s.db.QueryRow(ctx, `
		INSERT INTO jobs (`+jobColumns+`, stale, stale_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16,
			false, NULL, now())
		ON CONFLICT (dedup_key) DO UPDATE SET
			source = EXCLUDED.source,
			source_url = EXCLUDED.source_url,
			title = EXCLUDED.title,
			company = EXCLUDED.company,
			location = EXCLUDED.location,
			remote = EXCLUDED.remote,
			salary_stated_min = EXCLUDED.salary_stated_min,
			salary_stated_max = EXCLUDED.salary_stated_max,
			salary_currency = EXCLUDED.salary_currency,
			seniority = EXCLUDED.seniority,
			employment_type = EXCLUDED.employment_type,
			years_experience = EXCLUDED.years_experience,
			requirements = EXCLUDED.requirements,
			posted_at = EXCLUDED.posted_at,
			synthetic = EXCLUDED.synthetic,
			stale = false,
			stale_at = NULL,
			updated_at = now()
		RETURNING (xmax = 0) AS inserted`,
		j.Source, j.SourceURL, j.DedupKey, j.Title, j.Company, j.Location, j.Remote,
		j.SalaryStatedMin, j.SalaryStatedMax, currency, j.Seniority, j.EmploymentType,
		j.YearsExperience, string(reqs), j.PostedAt, j.Synthetic,
	).Scan(&inserted)
	if err != nil {
		return false, fmt.Errorf("upsert job: %w", err)
	}
	return inserted, nil
}

// ActiveJobs returns all non-stale jobs — the set the feed surfaces. It adapts
// the store to the feed's Provider interface.
func (s *PgxStore) ActiveJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE stale = false
		 ORDER BY posted_at DESC NULLS LAST, title, dedup_key`)
	if err != nil {
		return nil, fmt.Errorf("query active jobs: %w", err)
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return out, nil
}

// SearchFeed applies feed filters in Postgres and returns only the requested
// page. It is the optional production capability used by the feed HTTP service.
func (s *PgxStore) SearchFeed(ctx context.Context, q JobQuery) (JobPage, error) {
	if err := ValidateJobQuery(q); err != nil {
		return JobPage{}, fmt.Errorf("validate feed query: %w", err)
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	where, args := feedWhere(q)
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE `+where, args...).Scan(&total); err != nil {
		return JobPage{}, fmt.Errorf("count filtered jobs: %w", err)
	}

	limitParam := strconv.Itoa(len(args) + 1)
	offsetParam := strconv.Itoa(len(args) + 2)
	pageArgs := append(append([]any{}, args...), q.Limit, q.Offset)
	rows, err := s.db.Query(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE `+where+
			` ORDER BY posted_at DESC NULLS LAST, title, dedup_key
			LIMIT $`+limitParam+` OFFSET $`+offsetParam,
		pageArgs...,
	)
	if err != nil {
		return JobPage{}, fmt.Errorf("query filtered jobs: %w", err)
	}
	defer rows.Close()

	page := JobPage{Total: total, Jobs: make([]Job, 0, q.Limit)}
	for rows.Next() {
		j, scanErr := scanJob(rows)
		if scanErr != nil {
			return JobPage{}, fmt.Errorf("scan filtered job: %w", scanErr)
		}
		page.Jobs = append(page.Jobs, j)
	}
	if err := rows.Err(); err != nil {
		return JobPage{}, fmt.Errorf("iterate filtered jobs: %w", err)
	}
	return page, nil
}

func feedWhere(q JobQuery) (string, []any) {
	clauses := []string{"stale = false"}
	args := make([]any, 0, 8+len(q.Skills))
	param := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if search := strings.TrimSpace(q.Search); search != "" {
		p := param(search)
		clauses = append(clauses, `strpos(lower(title || ' ' || company), lower(`+p+`)) > 0`)
	}
	if q.PayMin != nil {
		p := param(*q.PayMin)
		clauses = append(clauses, `salary_stated_max IS NOT NULL AND salary_stated_max >= `+p)
	}
	if q.PayMax != nil {
		p := param(*q.PayMax)
		clauses = append(clauses, `salary_stated_min IS NOT NULL AND salary_stated_min <= `+p)
	}
	if location := strings.TrimSpace(q.Location); location != "" {
		p := param(location)
		clauses = append(clauses, `strpos(lower(COALESCE(location, '')), lower(`+p+`)) > 0`)
	}
	if q.Remote != nil {
		p := param(*q.Remote)
		clauses = append(clauses, `remote = `+p)
	}
	if q.EmploymentType != "" {
		p := param(q.EmploymentType)
		clauses = append(clauses, `employment_type = `+p)
	}
	if q.Source != "" {
		p := param(q.Source)
		clauses = append(clauses, `source = `+p)
	}
	if q.MaxYoE != nil {
		p := param(*q.MaxYoE)
		clauses = append(clauses, `(years_experience IS NULL OR years_experience <= `+p+`)`)
	}
	if q.PostedAfter != nil {
		p := param(*q.PostedAfter)
		clauses = append(clauses, `posted_at >= `+p)
	}
	for _, skill := range q.Skills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		p := param(skill)
		clauses = append(clauses,
			`EXISTS (SELECT 1 FROM jsonb_array_elements_text(requirements) AS req(value) `+
				`WHERE strpos(lower(req.value), lower(`+p+`)) > 0)`,
		)
	}
	return strings.Join(clauses, " AND "), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var (
		j    Job
		reqs []byte
	)
	if err := row.Scan(
		&j.Source, &j.SourceURL, &j.DedupKey, &j.Title, &j.Company, &j.Location, &j.Remote,
		&j.SalaryStatedMin, &j.SalaryStatedMax, &j.SalaryCurrency, &j.Seniority, &j.EmploymentType,
		&j.YearsExperience, &reqs, &j.PostedAt, &j.Synthetic,
	); err != nil {
		return Job{}, err
	}
	if err := ValidateSourceURL(j.SourceURL); err != nil {
		return Job{}, fmt.Errorf("validate job source url: %w", err)
	}
	if len(reqs) > 0 {
		if err := json.Unmarshal(reqs, &j.Requirements); err != nil {
			return Job{}, fmt.Errorf("unmarshal requirements: %w", err)
		}
	}
	return j, nil
}

// SweepStale marks jobs not re-seen within maxAge as stale (SCR-4) and returns
// the number newly marked. Already-stale jobs are not recounted.
func (s *PgxStore) SweepStale(ctx context.Context, maxAge time.Duration) (int, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE jobs SET stale = true, stale_at = now()
		 WHERE stale = false AND updated_at < now() - make_interval(secs => $1)`,
		maxAge.Seconds(),
	)
	if err != nil {
		return 0, fmt.Errorf("sweep stale: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RealActiveCount returns the number of active, non-synthetic jobs.
func (s *PgxStore) RealActiveCount(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE stale = false AND synthetic = false`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count real active jobs: %w", err)
	}
	return n, nil
}

var _ JobStore = (*PgxStore)(nil)
