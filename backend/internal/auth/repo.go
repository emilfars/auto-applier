package auth

import (
	"context"
	"time"
)

// User is an account record.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Verified     bool
	CreatedAt    time.Time
}

// Session is an issued login session.
type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// Repo is the persistence boundary for auth. Implementations may be in-memory
// (tests/dev) or Postgres-backed (production); the HTTP layer depends only on
// this interface.
type Repo interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, error)
	UserByID(ctx context.Context, id string) (User, error)
	SetVerified(ctx context.Context, userID string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error

	CreateVerification(ctx context.Context, userID, token string) error
	ConsumeVerification(ctx context.Context, token string) (userID string, err error)

	CreateSession(ctx context.Context, s Session) error
	SessionByToken(ctx context.Context, token string) (Session, error)
	DeleteSession(ctx context.Context, token string) error

	CreateReset(ctx context.Context, userID, token string, expiresAt time.Time) error
	ConsumeReset(ctx context.Context, token string, now time.Time) (userID string, err error)
}
