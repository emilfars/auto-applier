package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxDB is the subset of pgx the profile repo needs.
type PgxDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxRepo is the production Postgres-backed Repo, upserting the 1:1 profile row.
type PgxRepo struct {
	db PgxDB
}

// NewPgxRepo wraps a pgx pool/connection as a Repo.
func NewPgxRepo(db PgxDB) *PgxRepo { return &PgxRepo{db: db} }

const profileColumns = `full_name, email, phone, linkedin_url, github_url, portfolio_url,
	address, city, summary, current_employer, current_title, highest_education,
	education, work_history, skills,
	expected_salary, notice_period_days, work_authorization, open_to_relocation,
	preferred_locations, employment_type, confirmed, confirmed_at`

func (r *PgxRepo) Get(ctx context.Context, userID string) (Profile, error) {
	p := Profile{UserID: userID}
	err := r.db.QueryRow(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE user_id = $1`, userID,
	).Scan(
		&p.FullName, &p.Email, &p.Phone, &p.LinkedInURL, &p.GitHubURL, &p.PortfolioURL,
		&p.Address, &p.City, &p.Summary, &p.CurrentEmployer, &p.CurrentTitle, &p.HighestEducation,
		&p.Education, &p.WorkHistory, &p.Skills,
		&p.ExpectedSalary, &p.NoticePeriodDays, &p.WorkAuthorization, &p.OpenToRelocation,
		&p.PreferredLocations, &p.EmploymentType, &p.Confirmed, &p.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

func (r *PgxRepo) Save(ctx context.Context, p Profile) (Profile, error) {
	out := Profile{UserID: p.UserID}
	err := r.db.QueryRow(ctx, `
		INSERT INTO profiles (
			user_id, full_name, email, phone, linkedin_url, github_url, portfolio_url,
			address, city, summary, current_employer, current_title, highest_education,
			education, work_history, skills,
			expected_salary, notice_period_days, work_authorization, open_to_relocation,
			preferred_locations, employment_type, confirmed, confirmed_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14::jsonb, $15::jsonb, $16::jsonb,
			$17, $18, $19, $20, $21::jsonb, $22, $23, $24, now()
		)
		ON CONFLICT (user_id) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			email = EXCLUDED.email,
			phone = EXCLUDED.phone,
			linkedin_url = EXCLUDED.linkedin_url,
			github_url = EXCLUDED.github_url,
			portfolio_url = EXCLUDED.portfolio_url,
			address = EXCLUDED.address,
			city = EXCLUDED.city,
			summary = EXCLUDED.summary,
			current_employer = EXCLUDED.current_employer,
			current_title = EXCLUDED.current_title,
			highest_education = EXCLUDED.highest_education,
			education = EXCLUDED.education,
			work_history = EXCLUDED.work_history,
			skills = EXCLUDED.skills,
			expected_salary = EXCLUDED.expected_salary,
			notice_period_days = EXCLUDED.notice_period_days,
			work_authorization = EXCLUDED.work_authorization,
			open_to_relocation = EXCLUDED.open_to_relocation,
			preferred_locations = EXCLUDED.preferred_locations,
			employment_type = EXCLUDED.employment_type,
			confirmed = EXCLUDED.confirmed,
			confirmed_at = EXCLUDED.confirmed_at,
			updated_at = now()
		RETURNING `+profileColumns,
		p.UserID, p.FullName, p.Email, p.Phone, p.LinkedInURL, p.GitHubURL, p.PortfolioURL,
		p.Address, p.City, p.Summary, p.CurrentEmployer, p.CurrentTitle, p.HighestEducation,
		jsonbArray(p.Education), jsonbArray(p.WorkHistory), jsonbArray(p.Skills),
		p.ExpectedSalary, p.NoticePeriodDays, p.WorkAuthorization, p.OpenToRelocation,
		jsonbArray(p.PreferredLocations), p.EmploymentType, p.Confirmed, p.ConfirmedAt,
	).Scan(
		&out.FullName, &out.Email, &out.Phone, &out.LinkedInURL, &out.GitHubURL, &out.PortfolioURL,
		&out.Address, &out.City, &out.Summary, &out.CurrentEmployer, &out.CurrentTitle, &out.HighestEducation,
		&out.Education, &out.WorkHistory, &out.Skills,
		&out.ExpectedSalary, &out.NoticePeriodDays, &out.WorkAuthorization, &out.OpenToRelocation,
		&out.PreferredLocations, &out.EmploymentType, &out.Confirmed, &out.ConfirmedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("save profile: %w", err)
	}
	return out, nil
}

// Delete removes a user's profile. Idempotent: a missing profile is not an
// error (right to erasure, AC-AUTH-5).
func (r *PgxRepo) Delete(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM profiles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	return nil
}

// jsonbArray coerces an empty raw message to an empty JSON array so the jsonb
// column never receives invalid input.
func jsonbArray(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "[]"
	}
	return string(raw)
}

var _ Repo = (*PgxRepo)(nil)
