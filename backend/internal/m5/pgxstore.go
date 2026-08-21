package m5

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PgxDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PgxStore struct{ db PgxDB }

func NewPgxStore(db PgxDB) *PgxStore { return &PgxStore{db: db} }

// DeleteUser is idempotent. Foreign-key cascades remove the M5 rows when the
// account itself is deleted; this explicit delete also covers memory parity and
// makes account erasure safe if it is invoked before that final delete.
func (s *PgxStore) DeleteUser(ctx context.Context, userID string) error {
	for _, query := range []string{
		`DELETE FROM saved_filters WHERE user_id = $1`,
		`DELETE FROM job_user_states WHERE user_id = $1`,
		`DELETE FROM answer_snippets WHERE user_id = $1`,
		`DELETE FROM applications WHERE user_id = $1`,
	} {
		if _, err := s.db.Exec(ctx, query, userID); err != nil {
			return fmt.Errorf("delete m5 data: %w", err)
		}
	}
	return nil
}

func (s *PgxStore) ListSavedFilterUsers(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT user_id::text FROM saved_filters`)
	if err != nil {
		return nil, fmt.Errorf("list saved filter users: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan saved filter user: %w", err)
		}
		out = append(out, userID)
	}
	return out, rows.Err()
}

func (s *PgxStore) ExportUser(ctx context.Context, userID string) (UserData, error) {
	filters, err := s.ListSavedFilters(ctx, userID)
	if err != nil {
		return UserData{}, err
	}
	states, err := s.listJobStates(ctx, userID)
	if err != nil {
		return UserData{}, err
	}
	applications, err := s.ListApplications(ctx, userID)
	if err != nil {
		return UserData{}, err
	}
	snippets, err := s.ListSnippets(ctx, userID)
	if err != nil {
		return UserData{}, err
	}
	return UserData{SavedFilters: filters, JobStates: states, Applications: applications, Snippets: snippets}, nil
}

func (s *PgxStore) listJobStates(ctx context.Context, userID string) ([]JobState, error) {
	rows, err := s.db.Query(ctx, `SELECT job_key, dismissed, already_applied, updated_at FROM job_user_states WHERE user_id = $1 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list job states: %w", err)
	}
	defer rows.Close()
	var out []JobState
	for rows.Next() {
		var state JobState
		if err := rows.Scan(&state.JobKey, &state.Dismissed, &state.AlreadyApplied, &state.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan job state: %w", err)
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

const filterColumns = `id::text, user_id::text, name, query, created_at, last_alerted_at`

func decodeQuery(raw []byte) (ingest.JobQuery, error) {
	var q ingest.JobQuery
	if err := json.Unmarshal(raw, &q); err != nil {
		return ingest.JobQuery{}, fmt.Errorf("decode saved filter: %w", err)
	}
	return q, nil
}

func scanFilter(row interface{ Scan(...any) error }) (SavedFilter, error) {
	var f SavedFilter
	var raw []byte
	if err := row.Scan(&f.ID, &f.UserID, &f.Name, &raw, &f.CreatedAt, &f.LastAlertedAt); err != nil {
		return SavedFilter{}, err
	}
	var err error
	f.Query, err = decodeQuery(raw)
	return f, err
}

func (s *PgxStore) ListSavedFilters(ctx context.Context, userID string) ([]SavedFilter, error) {
	rows, err := s.db.Query(ctx, `SELECT `+filterColumns+` FROM saved_filters WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list saved filters: %w", err)
	}
	defer rows.Close()
	var out []SavedFilter
	for rows.Next() {
		f, scanErr := scanFilter(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan saved filter: %w", scanErr)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *PgxStore) GetSavedFilter(ctx context.Context, userID, id string) (SavedFilter, error) {
	f, err := scanFilter(s.db.QueryRow(ctx, `SELECT `+filterColumns+` FROM saved_filters WHERE user_id = $1 AND id = $2`, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedFilter{}, ErrNotFound
	}
	if err != nil {
		return SavedFilter{}, fmt.Errorf("get saved filter: %w", err)
	}
	return f, nil
}

func (s *PgxStore) CreateSavedFilter(ctx context.Context, f SavedFilter) (SavedFilter, error) {
	raw, err := json.Marshal(f.Query)
	if err != nil {
		return SavedFilter{}, fmt.Errorf("encode saved filter: %w", err)
	}
	created, err := scanFilter(s.db.QueryRow(ctx, `
		INSERT INTO saved_filters (user_id, name, query) VALUES ($1, $2, $3::jsonb)
		RETURNING `+filterColumns, f.UserID, strings.TrimSpace(f.Name), string(raw)))
	if err != nil {
		if strings.Contains(err.Error(), "saved_filters_user_name_key") {
			return SavedFilter{}, ErrConflict
		}
		return SavedFilter{}, fmt.Errorf("create saved filter: %w", err)
	}
	return created, nil
}

func (s *PgxStore) DeleteSavedFilter(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM saved_filters WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return fmt.Errorf("delete saved filter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PgxStore) TouchSavedFilter(ctx context.Context, userID, id string, at time.Time) error {
	tag, err := s.db.Exec(ctx, `UPDATE saved_filters SET last_alerted_at = $3 WHERE user_id = $1 AND id = $2`, userID, id, at)
	if err != nil {
		return fmt.Errorf("touch saved filter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PgxStore) GetJobState(ctx context.Context, userID, key string) (JobState, error) {
	var state JobState
	err := s.db.QueryRow(ctx, `SELECT job_key, dismissed, already_applied, updated_at FROM job_user_states WHERE user_id = $1 AND job_key = $2`, userID, key).
		Scan(&state.JobKey, &state.Dismissed, &state.AlreadyApplied, &state.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobState{JobKey: key}, nil
	}
	if err != nil {
		return JobState{}, fmt.Errorf("get job state: %w", err)
	}
	return state, nil
}

func (s *PgxStore) SetJobState(ctx context.Context, userID string, state JobState) (JobState, error) {
	err := s.db.QueryRow(ctx, `
		INSERT INTO job_user_states (user_id, job_key, dismissed, already_applied, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id, job_key) DO UPDATE SET
			dismissed = EXCLUDED.dismissed, already_applied = EXCLUDED.already_applied, updated_at = now()
		RETURNING job_key, dismissed, already_applied, updated_at`,
		userID, state.JobKey, state.Dismissed, state.AlreadyApplied).
		Scan(&state.JobKey, &state.Dismissed, &state.AlreadyApplied, &state.UpdatedAt)
	if err != nil {
		return JobState{}, fmt.Errorf("set job state: %w", err)
	}
	return state, nil
}

const applicationSelect = `
	SELECT a.id::text, a.user_id::text, j.dedup_key, j.title, j.company,
		a.status, a.created_at, a.updated_at
	FROM applications a JOIN jobs j ON j.id = a.job_id`

func scanApplication(row interface{ Scan(...any) error }) (Application, error) {
	var app Application
	err := row.Scan(&app.ID, &app.UserID, &app.JobKey, &app.JobTitle, &app.Company, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	return app, err
}

func (s *PgxStore) ListApplications(ctx context.Context, userID string) ([]Application, error) {
	rows, err := s.db.Query(ctx, applicationSelect+` WHERE a.user_id = $1 ORDER BY a.updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		app, scanErr := scanApplication(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan application: %w", scanErr)
		}
		out = append(out, app)
	}
	return out, rows.Err()
}

func (s *PgxStore) GetApplication(ctx context.Context, userID, id string) (Application, error) {
	app, err := scanApplication(s.db.QueryRow(ctx, applicationSelect+` WHERE a.user_id = $1 AND a.id = $2`, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrNotFound
	}
	if err != nil {
		return Application{}, fmt.Errorf("get application: %w", err)
	}
	return app, nil
}

func (s *PgxStore) CreateApplication(ctx context.Context, app Application) (Application, error) {
	var jobID string
	if err := s.db.QueryRow(ctx, `SELECT id::text FROM jobs WHERE dedup_key = $1`, app.JobKey).Scan(&jobID); errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrJobNotFound
	} else if err != nil {
		return Application{}, fmt.Errorf("find application job: %w", err)
	}
	var id string
	if err := s.db.QueryRow(ctx, `
		INSERT INTO applications (user_id, job_id, status) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, job_id) DO UPDATE SET updated_at = applications.updated_at
		RETURNING id::text`, app.UserID, jobID, app.Status).Scan(&id); err != nil {
		return Application{}, fmt.Errorf("create application: %w", err)
	}
	return s.GetApplication(ctx, app.UserID, id)
}

func (s *PgxStore) UpdateApplication(ctx context.Context, userID string, app Application) (Application, error) {
	tag, err := s.db.Exec(ctx, `UPDATE applications SET status = $3, updated_at = now() WHERE user_id = $1 AND id = $2`, userID, app.ID, app.Status)
	if err != nil {
		return Application{}, fmt.Errorf("update application: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Application{}, ErrNotFound
	}
	return s.GetApplication(ctx, userID, app.ID)
}

const snippetColumns = `id::text, user_id::text, name, body, created_at, updated_at`

func scanSnippet(row interface{ Scan(...any) error }) (Snippet, error) {
	var snippet Snippet
	err := row.Scan(&snippet.ID, &snippet.UserID, &snippet.Name, &snippet.Body, &snippet.CreatedAt, &snippet.UpdatedAt)
	return snippet, err
}

func (s *PgxStore) ListSnippets(ctx context.Context, userID string) ([]Snippet, error) {
	rows, err := s.db.Query(ctx, `SELECT `+snippetColumns+` FROM answer_snippets WHERE user_id = $1 ORDER BY name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list snippets: %w", err)
	}
	defer rows.Close()
	var out []Snippet
	for rows.Next() {
		snippet, scanErr := scanSnippet(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan snippet: %w", scanErr)
		}
		out = append(out, snippet)
	}
	return out, rows.Err()
}

func (s *PgxStore) GetSnippet(ctx context.Context, userID, id string) (Snippet, error) {
	snippet, err := scanSnippet(s.db.QueryRow(ctx, `SELECT `+snippetColumns+` FROM answer_snippets WHERE user_id = $1 AND id = $2`, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Snippet{}, ErrNotFound
	}
	if err != nil {
		return Snippet{}, fmt.Errorf("get snippet: %w", err)
	}
	return snippet, nil
}

func (s *PgxStore) CreateSnippet(ctx context.Context, snippet Snippet) (Snippet, error) {
	created, err := scanSnippet(s.db.QueryRow(ctx, `
		INSERT INTO answer_snippets (user_id, name, body) VALUES ($1, $2, $3)
		RETURNING `+snippetColumns, snippet.UserID, strings.TrimSpace(snippet.Name), snippet.Body))
	if err != nil {
		if strings.Contains(err.Error(), "answer_snippets_user_name_key") {
			return Snippet{}, ErrConflict
		}
		return Snippet{}, fmt.Errorf("create snippet: %w", err)
	}
	return created, nil
}

func (s *PgxStore) UpdateSnippet(ctx context.Context, userID string, snippet Snippet) (Snippet, error) {
	updated, err := scanSnippet(s.db.QueryRow(ctx, `
		UPDATE answer_snippets SET name = $3, body = $4, updated_at = now()
		WHERE user_id = $1 AND id = $2 RETURNING `+snippetColumns,
		userID, snippet.ID, strings.TrimSpace(snippet.Name), snippet.Body))
	if errors.Is(err, pgx.ErrNoRows) {
		return Snippet{}, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "answer_snippets_user_name_key") {
			return Snippet{}, ErrConflict
		}
		return Snippet{}, fmt.Errorf("update snippet: %w", err)
	}
	return updated, nil
}

func (s *PgxStore) DeleteSnippet(ctx context.Context, userID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM answer_snippets WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return fmt.Errorf("delete snippet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var _ Store = (*PgxStore)(nil)
