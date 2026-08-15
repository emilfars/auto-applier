package profile

import (
	"context"
	"encoding/json"

	"github.com/auto-applier/backend/internal/cv"
)

// ApplyParsedCV persists a parsed CV into the user's profile (AC-CV-2). It fills
// the contact and CV-derived fields while preserving any added-info the user
// already entered. Crucially it leaves the profile unconfirmed: parsed data is
// never authoritative and the user must review + confirm it before any fill
// flow may be armed (AC-CV-3/5, Prime Directive).
func (s *Service) ApplyParsedCV(ctx context.Context, userID string, parsed cv.ParsedCV) error {
	p, err := s.repo.Get(ctx, userID)
	if err == ErrNotFound {
		p = emptyProfile(userID)
	} else if err != nil {
		return err
	}

	if parsed.FullName != "" {
		p.FullName = parsed.FullName
	}
	if parsed.Email != "" {
		p.Email = parsed.Email
	}
	if parsed.Phone != "" {
		p.Phone = parsed.Phone
	}
	p.Education = marshalArray(parsed.Education)
	p.WorkHistory = marshalArray(parsed.WorkHistory)
	p.Skills = marshalArray(parsed.Skills)

	// New parsed data invalidates any prior confirmation: re-review required.
	p.Confirmed = false
	p.ConfirmedAt = nil

	_, err = s.repo.Save(ctx, p)
	return err
}

// marshalArray marshals v to a JSON array, falling back to an empty array so the
// profile's array invariants hold even for nil/empty input.
func marshalArray(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return json.RawMessage("[]")
	}
	return b
}
