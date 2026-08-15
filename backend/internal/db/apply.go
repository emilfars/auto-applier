package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is the minimal query surface used inside a migration transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type beginner interface {
	Execer
	Begin(context.Context) (pgx.Tx, error)
}

// Apply runs any not-yet-applied migrations in ascending version order and
// records them in schema_migrations. It is idempotent: already-applied
// versions are skipped. Returns the number of migrations applied.
func Apply(ctx context.Context, db beginner) (int, error) {
	migs, err := LoadMigrations()
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7022647215679107180)`); err != nil {
		return 0, fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied := 0
	for _, m := range migs {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, m.Version,
		).Scan(&exists); err != nil {
			return 0, fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if exists {
			continue
		}
		if _, err := tx.Exec(ctx, m.Up); err != nil {
			return 0, fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name,
		); err != nil {
			return 0, fmt.Errorf("record migration %d: %w", m.Version, err)
		}
		applied++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit migrations: %w", err)
	}
	return applied, nil
}
