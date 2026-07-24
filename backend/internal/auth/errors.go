package auth

import "errors"

var (
	// ErrEmailTaken is returned when registering an already-used email.
	ErrEmailTaken = errors.New("auth: email already registered")
	// ErrNotFound is returned when a user/session/token does not exist.
	ErrNotFound = errors.New("auth: not found")
	// ErrInvalidToken is returned for missing, expired, or unknown tokens.
	ErrInvalidToken = errors.New("auth: invalid token")
)
