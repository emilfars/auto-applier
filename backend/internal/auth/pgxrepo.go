package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxDB is the subset of pgx we depend on. Both *pgxpool.Pool and *pgx.Conn
// satisfy it, so the repo works with a pooled or single connection.
type PgxDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgxRepo is the production Postgres-backed Repo. UUID columns are cast to text
// so ids scan into plain strings, matching the in-memory repo's contract.
type PgxRepo struct {
	db PgxDB
}

// NewPgxRepo wraps a pgx pool/connection as a Repo.
func NewPgxRepo(db PgxDB) *PgxRepo { return &PgxRepo{db: db} }

const uniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

func (r *PgxRepo) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id::text, email, verified, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.Verified, &u.CreatedAt)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	u.PasswordHash = passwordHash
	return u, nil
}

func (r *PgxRepo) UserByEmail(ctx context.Context, email string) (User, error) {
	return r.scanUser(ctx,
		`SELECT id::text, email, password_hash, verified, created_at
		 FROM users WHERE lower(email) = lower($1)`, email)
}

func (r *PgxRepo) UserByID(ctx context.Context, id string) (User, error) {
	return r.scanUser(ctx,
		`SELECT id::text, email, password_hash, verified, created_at
		 FROM users WHERE id = $1`, id)
}

func (r *PgxRepo) scanUser(ctx context.Context, query string, arg any) (User, error) {
	var u User
	var hash *string
	err := r.db.QueryRow(ctx, query, arg).Scan(&u.ID, &u.Email, &hash, &u.Verified, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	if hash != nil {
		u.PasswordHash = *hash
	}
	return u, nil
}

func (r *PgxRepo) SetVerified(ctx context.Context, userID string) error {
	return r.execAffecting(ctx,
		`UPDATE users SET verified = TRUE, updated_at = now() WHERE id = $1`, userID)
}

func (r *PgxRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return r.execAffecting(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, passwordHash)
}

// execAffecting runs a statement expected to touch exactly one row, mapping a
// zero row count to ErrNotFound.
func (r *PgxRepo) execAffecting(ctx context.Context, query string, args ...any) error {
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgxRepo) CreateVerification(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO email_verifications (token, user_id) VALUES ($1, $2)`, token, userID)
	if err != nil {
		return fmt.Errorf("create verification: %w", err)
	}
	return nil
}

func (r *PgxRepo) ConsumeVerification(ctx context.Context, token string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx,
		`DELETE FROM email_verifications WHERE token = $1 RETURNING user_id::text`, token,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("consume verification: %w", err)
	}
	return userID, nil
}

func (r *PgxRepo) CreateSession(ctx context.Context, s Session) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		s.Token, s.UserID, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *PgxRepo) SessionByToken(ctx context.Context, token string) (Session, error) {
	var s Session
	err := r.db.QueryRow(ctx,
		`SELECT token, user_id::text, expires_at FROM sessions WHERE token = $1`, token,
	).Scan(&s.Token, &s.UserID, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("session by token: %w", err)
	}
	return s, nil
}

func (r *PgxRepo) DeleteSession(ctx context.Context, token string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *PgxRepo) CreateReset(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO password_resets (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create reset: %w", err)
	}
	return nil
}

func (r *PgxRepo) ConsumeReset(ctx context.Context, token string, now time.Time) (string, error) {
	var userID string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`DELETE FROM password_resets WHERE token = $1 RETURNING user_id::text, expires_at`, token,
	).Scan(&userID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("consume reset: %w", err)
	}
	if now.After(expiresAt) {
		return "", ErrInvalidToken
	}
	return userID, nil
}

// compile-time assertion that PgxRepo satisfies Repo.
var _ Repo = (*PgxRepo)(nil)
