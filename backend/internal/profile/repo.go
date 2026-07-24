// Package profile manages a user's structured, self-reviewed CV profile and the
// confirm-before-apply gate (AC-CV-5). Per the Prime Directive the system never
// submits an application; the confirm gate exists to ensure the user has
// reviewed their data before any fill flow may be armed.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when a user has no profile yet.
var ErrNotFound = errors.New("profile: not found")

// Profile is a user's structured profile. The JSON-array fields hold
// user-reviewed CV data; they default to an empty array.
type Profile struct {
	UserID             string
	FullName           string
	Phone              string
	Education          json.RawMessage
	WorkHistory        json.RawMessage
	Skills             json.RawMessage
	ExpectedSalary     *int64
	NoticePeriodDays   *int
	WorkAuthorization  string
	OpenToRelocation   bool
	PreferredLocations json.RawMessage
	EmploymentType     string
	// Confirmed gates the fill flow: it starts false and is set true only by an
	// explicit user confirm. Any edit resets it to false (re-review required).
	Confirmed   bool
	ConfirmedAt *time.Time
}

// emptyProfile returns a zero profile with JSON-array fields initialised.
func emptyProfile(userID string) Profile {
	return Profile{
		UserID:             userID,
		Education:          json.RawMessage("[]"),
		WorkHistory:        json.RawMessage("[]"),
		Skills:             json.RawMessage("[]"),
		PreferredLocations: json.RawMessage("[]"),
	}
}

// CanArm reports whether the fill flow may be armed for this profile. It is the
// single source of truth for the confirm-before-apply gate.
func (p Profile) CanArm() bool { return p.Confirmed }

// Repo persists profiles (1:1 with users). Implementations may be in-memory
// (tests/dev) or Postgres-backed (production).
type Repo interface {
	Get(ctx context.Context, userID string) (Profile, error)
	Save(ctx context.Context, p Profile) (Profile, error)
	// Delete removes a user's profile. It is idempotent: deleting a
	// non-existent profile is not an error (right to erasure, AC-AUTH-5).
	Delete(ctx context.Context, userID string) error
}
