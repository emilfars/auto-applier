package cv

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxDB is the subset of pgx the CV repo needs.
type PgxDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxRepo is the production Postgres-backed CV metadata repo. Only object-store
// references and metadata live in Postgres — never CV contents/PII (AC-CV-1b).
type PgxRepo struct {
	db PgxDB
}

// NewPgxRepo wraps a pgx pool/connection as a Repo.
func NewPgxRepo(db PgxDB) *PgxRepo { return &PgxRepo{db: db} }

const cvColumns = `id::text, user_id::text, object_key, filename, label, is_primary, content_type, size_bytes, created_at`

func (r *PgxRepo) CreateFile(ctx context.Context, f File) (File, error) {
	var out File
	err := r.db.QueryRow(ctx, `
		INSERT INTO cv_files (user_id, object_key, filename, label, is_primary, content_type, size_bytes, encrypted)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), $3),
			$5 OR NOT EXISTS (SELECT 1 FROM cv_files WHERE user_id = $1), $6, $7, true)
		RETURNING `+cvColumns,
		f.UserID, f.ObjectKey, f.Filename, f.Label, f.IsPrimary, f.ContentType, f.SizeBytes,
	).Scan(&out.ID, &out.UserID, &out.ObjectKey, &out.Filename, &out.Label, &out.IsPrimary, &out.ContentType, &out.SizeBytes, &out.CreatedAt)
	if err != nil {
		return File{}, fmt.Errorf("create cv file: %w", err)
	}
	return out, nil
}

func (r *PgxRepo) FileByID(ctx context.Context, id string) (File, error) {
	var f File
	err := r.db.QueryRow(ctx,
		`SELECT `+cvColumns+` FROM cv_files WHERE id = $1`, id,
	).Scan(&f.ID, &f.UserID, &f.ObjectKey, &f.Filename, &f.Label, &f.IsPrimary, &f.ContentType, &f.SizeBytes, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, fmt.Errorf("get cv file: %w", err)
	}
	return f, nil
}

func (r *PgxRepo) FilesByUser(ctx context.Context, userID string) ([]File, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+cvColumns+` FROM cv_files WHERE user_id = $1 ORDER BY is_primary DESC, created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list cv files: %w", err)
	}
	defer rows.Close()

	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.UserID, &f.ObjectKey, &f.Filename, &f.Label, &f.IsPrimary, &f.ContentType, &f.SizeBytes, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cv file: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cv files: %w", err)
	}
	return out, nil
}

// DeleteByUser removes all of a user's CV metadata rows. Idempotent (right to
// erasure, AC-AUTH-5). Callers delete the referenced objects from storage.
func (r *PgxRepo) DeleteByUser(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM cv_files WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete cv files: %w", err)
	}
	return nil
}

func (r *PgxRepo) UpdateVersion(ctx context.Context, userID, id, label string, primary *bool) (File, error) {
	var owned bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cv_files WHERE user_id = $1 AND id = $2)`, userID, id).Scan(&owned); err != nil {
		return File{}, fmt.Errorf("check cv version: %w", err)
	}
	if !owned {
		return File{}, ErrNotFound
	}
	if primary != nil && *primary {
		if _, err := r.db.Exec(ctx, `UPDATE cv_files SET is_primary = false WHERE user_id = $1`, userID); err != nil {
			return File{}, fmt.Errorf("clear primary cv: %w", err)
		}
	}
	args := []any{userID, id}
	set := ""
	if strings.TrimSpace(label) != "" {
		set = "label = $3"
		args = append(args, strings.TrimSpace(label))
	}
	if primary != nil {
		if set != "" {
			set += ", "
		}
		set += "is_primary = $" + strconv.Itoa(len(args)+1)
		args = append(args, *primary)
	}
	if set == "" {
		return File{}, ErrNotFound
	}
	query := `UPDATE cv_files SET ` + set + ` WHERE user_id = $1 AND id = $2 RETURNING ` + cvColumns
	var f File
	err := r.db.QueryRow(ctx, query, args...).Scan(&f.ID, &f.UserID, &f.ObjectKey, &f.Filename, &f.Label, &f.IsPrimary, &f.ContentType, &f.SizeBytes, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, fmt.Errorf("update cv version: %w", err)
	}
	return f, nil
}

var _ Repo = (*PgxRepo)(nil)
