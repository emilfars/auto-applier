package ingest

import (
	"context"
	"encoding/json"
	"fmt"
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
	years_experience, requirements, posted_at`

// Upsert inserts or updates a job keyed by dedup_key (SCR-3). Re-seeing a
// listing refreshes updated_at (last-seen) and clears any stale flag (SCR-4).
// It returns whether the row was newly created (vs. an update).
func (s *PgxStore) Upsert(ctx context.Context, j Job) (bool, error) {
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15,
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
			stale = false,
			stale_at = NULL,
			updated_at = now()
		RETURNING (xmax = 0) AS inserted`,
		j.Source, j.SourceURL, j.DedupKey, j.Title, j.Company, j.Location, j.Remote,
		j.SalaryStatedMin, j.SalaryStatedMax, currency, j.Seniority, j.EmploymentType,
		j.YearsExperience, string(reqs), j.PostedAt,
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
		 ORDER BY posted_at DESC NULLS LAST, title`)
	if err != nil {
		return nil, fmt.Errorf("query active jobs: %w", err)
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var (
			j    Job
			reqs []byte
		)
		if err := rows.Scan(
			&j.Source, &j.SourceURL, &j.DedupKey, &j.Title, &j.Company, &j.Location, &j.Remote,
			&j.SalaryStatedMin, &j.SalaryStatedMax, &j.SalaryCurrency, &j.Seniority, &j.EmploymentType,
			&j.YearsExperience, &reqs, &j.PostedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		if len(reqs) > 0 {
			if err := json.Unmarshal(reqs, &j.Requirements); err != nil {
				return nil, fmt.Errorf("unmarshal requirements: %w", err)
			}
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return out, nil
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

// Count returns the number of non-stale jobs (used by seed/health checks).
func (s *PgxStore) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE stale = false`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}
	return n, nil
}

var _ JobStore = (*PgxStore)(nil)
