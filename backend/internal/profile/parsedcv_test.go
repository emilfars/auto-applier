package profile

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/cv"
)

// TestApplyParsedCV verifies parsed data lands in the profile, array fields are
// stored as JSON arrays, and the profile is left unconfirmed so the user must
// review before arming (AC-CV-2/5).
func TestApplyParsedCV(t *testing.T) {
	repo := NewMemoryRepo()
	svc := NewService(repo, func() time.Time { return time.Unix(0, 0) })

	// Pre-confirm to prove parsing resets it.
	confirmed := time.Unix(100, 0)
	if _, err := repo.Save(context.Background(), Profile{
		UserID: "u1", Confirmed: true, ConfirmedAt: &confirmed,
		Education: json.RawMessage("[]"), WorkHistory: json.RawMessage("[]"),
		Skills: json.RawMessage("[]"), PreferredLocations: json.RawMessage("[]"),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	parsed := cv.ParsedCV{
		FullName: "Budi Santoso",
		Email:    "budi@example.com",
		Phone:    "+6281234567890",
		Education: []cv.EducationEntry{
			{Institution: "Universitas Indonesia", Degree: "Sarjana Teknik"},
		},
		WorkHistory: []cv.WorkEntry{{Company: "PT Tokopedia", Title: "Backend Engineer"}},
		Skills:      []string{"Go", "React"},
	}
	if err := svc.ApplyParsedCV(context.Background(), "u1", parsed); err != nil {
		t.Fatalf("ApplyParsedCV: %v", err)
	}

	got, err := repo.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FullName != "Budi Santoso" || got.Email != "budi@example.com" || got.Phone != "+6281234567890" {
		t.Errorf("contact not applied: %+v", got)
	}
	if got.Confirmed || got.ConfirmedAt != nil {
		t.Error("profile must be unconfirmed after parsing (re-review required)")
	}
	var skills []string
	if err := json.Unmarshal(got.Skills, &skills); err != nil {
		t.Fatalf("skills not a JSON array: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("skills = %v, want 2 entries", skills)
	}
	if !isJSONArray(got.Education) || !isJSONArray(got.WorkHistory) {
		t.Error("education/work_history must be JSON arrays")
	}
}

// Parsing when no profile exists yet should create one.
func TestApplyParsedCVCreatesProfile(t *testing.T) {
	repo := NewMemoryRepo()
	svc := NewService(repo, nil)
	if err := svc.ApplyParsedCV(context.Background(), "new-user", cv.ParsedCV{FullName: "Jane Doe"}); err != nil {
		t.Fatalf("ApplyParsedCV: %v", err)
	}
	got, err := repo.Get(context.Background(), "new-user")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FullName != "Jane Doe" || got.Confirmed {
		t.Errorf("unexpected profile: %+v", got)
	}
}
