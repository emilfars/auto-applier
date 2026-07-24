package profile

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
)

type harness struct {
	handler http.Handler
}

func newHarness() *harness {
	svc := NewService(NewMemoryRepo(), func() time.Time {
		return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	})
	h := &harness{}
	h.handler = withUser(svc.Routes(), auth.User{ID: "user-1", Email: "u@example.com", Verified: true})
	return h
}

func withUser(next http.Handler, u auth.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(auth.ContextWithUser(r.Context(), u)))
	})
}

func (h *harness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func decodeProfile(t *testing.T, rec *httptest.ResponseRecorder) profileResp {
	t.Helper()
	var p profileResp
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode profile: %v (%s)", err, rec.Body.String())
	}
	return p
}

// AC-CV-3: parsed fields are editable and persist.
func TestAC_CV_3_EditPersists(t *testing.T) {
	h := newHarness()
	patch := map[string]any{
		"full_name":    "Dina Putri",
		"phone":        "+628123456789",
		"education":    []map[string]any{{"school": "UI", "degree": "S.Kom"}},
		"work_history": []map[string]any{{"company": "Tokopedia", "role": "SWE"}},
		"skills":       []string{"Go", "React"},
	}
	if rec := h.do(t, http.MethodPatch, "/profile", patch); rec.Code != http.StatusOK {
		t.Fatalf("patch: got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	rec := h.do(t, http.MethodGet, "/profile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", rec.Code)
	}
	p := decodeProfile(t, rec)
	if p.FullName != "Dina Putri" || p.Phone != "+628123456789" {
		t.Fatalf("fields not persisted: %+v", p)
	}
	if string(p.Skills) != `["Go","React"]` {
		t.Fatalf("skills not persisted: %s", p.Skills)
	}
}

// AC-CV-4: added-info fields persist and validate.
func TestAC_CV_4_AddedInfoValidation(t *testing.T) {
	h := newHarness()

	// valid added-info persists
	salary := int64(15000000)
	notice := 30
	patch := map[string]any{
		"expected_salary":     salary,
		"notice_period_days":  notice,
		"work_authorization":  "WNI",
		"open_to_relocation":  true,
		"preferred_locations": []string{"Jakarta", "Bandung"},
		"employment_type":     "full_time",
	}
	rec := h.do(t, http.MethodPatch, "/profile", patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid patch: got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	p := decodeProfile(t, rec)
	if p.ExpectedSalary == nil || *p.ExpectedSalary != salary {
		t.Fatalf("expected_salary not persisted: %+v", p.ExpectedSalary)
	}
	if p.EmploymentType != "full_time" || !p.OpenToRelocation {
		t.Fatalf("added-info not persisted: %+v", p)
	}

	// validation failures → 400
	bad := []map[string]any{
		{"expected_salary": -1},
		{"notice_period_days": 400},
		{"employment_type": "wizard"},
		{"skills": "not-an-array"},
	}
	for _, b := range bad {
		if rec := h.do(t, http.MethodPatch, "/profile", b); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad patch %v: got %d, want 400", b, rec.Code)
		}
	}
}

// AC-CV-5: confirm-before-apply gate. Arm is refused until the user confirms;
// editing after confirming re-locks the gate.
func TestAC_CV_5_ConfirmBeforeApplyGate(t *testing.T) {
	h := newHarness()

	// Fresh profile: unconfirmed, arm refused.
	if rec := h.do(t, http.MethodGet, "/profile/arm", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("arm before confirm: got %d, want 403", rec.Code)
	}

	// Add data, then confirm.
	h.do(t, http.MethodPatch, "/profile", map[string]any{"full_name": "Dina"})
	rec := h.do(t, http.MethodPost, "/profile/confirm", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm: got %d, want 200", rec.Code)
	}
	if p := decodeProfile(t, rec); !p.Confirmed || p.ConfirmedAt == nil {
		t.Fatalf("profile not marked confirmed: %+v", p)
	}

	// Now arm is allowed.
	if rec := h.do(t, http.MethodGet, "/profile/arm", nil); rec.Code != http.StatusOK {
		t.Fatalf("arm after confirm: got %d, want 200", rec.Code)
	}

	// Editing re-locks the gate (must re-review).
	h.do(t, http.MethodPatch, "/profile", map[string]any{"phone": "+628100000000"})
	if rec := h.do(t, http.MethodGet, "/profile/arm", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("arm after edit: got %d, want 403 (re-review required)", rec.Code)
	}
	rec = h.do(t, http.MethodGet, "/profile", nil)
	if p := decodeProfile(t, rec); p.Confirmed {
		t.Fatal("edit did not reset confirmed")
	}
}

func TestProfileRequiresAuth(t *testing.T) {
	svc := NewService(NewMemoryRepo(), nil)
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: got %d, want 401", rec.Code)
	}
}

// Default profile exposes empty JSON arrays, never null.
func TestDefaultProfileArrays(t *testing.T) {
	h := newHarness()
	rec := h.do(t, http.MethodGet, "/profile", nil)
	p := decodeProfile(t, rec)
	for _, f := range []json.RawMessage{p.Education, p.WorkHistory, p.Skills, p.PreferredLocations} {
		if string(f) != "[]" {
			t.Fatalf("default array field = %s, want []", f)
		}
	}
}
