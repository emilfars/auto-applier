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
	// ConsentAt is when the user consented to data processing at signup
	// (UU PDP No. 27/2022). Zero means no consent has been recorded.
	ConsentAt time.Time
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
	SetConsent(ctx context.Context, userID string, at time.Time) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	// DeleteUser removes the account and all rows that reference it
	// (sessions, verifications, resets, profile, cv metadata). Right to
	// erasure under UU PDP (AC-AUTH-5).
	DeleteUser(ctx context.Context, userID string) error

	CreateVerification(ctx context.Context, userID, token string) error
	ConsumeVerification(ctx context.Context, token string) (userID string, err error)

	CreateSession(ctx context.Context, s Session) error
	SessionByToken(ctx context.Context, token string) (Session, error)
	DeleteSession(ctx context.Context, token string) error

	CreateReset(ctx context.Context, userID, token string, expiresAt time.Time) error
	ConsumeReset(ctx context.Context, token string, now time.Time) (userID string, err error)
}
