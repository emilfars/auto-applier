package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
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
		"full_name":         "Dina Putri",
		"email":             "dina@example.com",
		"phone":             "+628123456789",
		"linkedin_url":      "https://linkedin.com/in/dina",
		"github_url":        "https://github.com/dina",
		"portfolio_url":     "https://dina.example.com",
		"address":           "Jl. Merdeka 1",
		"city":              "Jakarta",
		"summary":           "Product-minded engineer",
		"current_employer":  "Acme",
		"current_title":     "SWE",
		"highest_education": "S.Kom",
		"education":         []map[string]any{{"institution": "UI", "degree": "S.Kom"}},
		"work_history":      []map[string]any{{"company": "Tokopedia", "title": "SWE"}},
		"skills":            []string{"Go", "React"},
	}
	if rec := h.do(t, http.MethodPatch, "/profile", patch); rec.Code != http.StatusOK {
		t.Fatalf("patch: got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	rec := h.do(t, http.MethodGet, "/profile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", rec.Code)
	}
	p := decodeProfile(t, rec)
	if p.FullName != "Dina Putri" || p.Email != "dina@example.com" || p.Phone != "+628123456789" ||
		p.LinkedInURL != "https://linkedin.com/in/dina" || p.GitHubURL != "https://github.com/dina" ||
		p.PortfolioURL != "https://dina.example.com" || p.Address != "Jl. Merdeka 1" ||
		p.City != "Jakarta" || p.Summary != "Product-minded engineer" ||
		p.CurrentEmployer != "Acme" || p.CurrentTitle != "SWE" ||
		p.HighestEducation != "S.Kom" {
		t.Fatalf("fields not persisted: %+v", p)
	}
	if string(p.Education) != `[{"degree":"S.Kom","institution":"UI"}]` ||
		string(p.WorkHistory) != `[{"company":"Tokopedia","title":"SWE"}]` {
		t.Fatalf("structured fields not persisted: education=%s work_history=%s", p.Education, p.WorkHistory)
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
		{"skills": []int{1}},
		{"education": []map[string]any{{"school": "unknown field"}}},
		{"email": "not-an-email"},
	}
	for _, b := range bad {
		if rec := h.do(t, http.MethodPatch, "/profile", b); rec.Code != http.StatusBadRequest {
			t.Fatalf("bad patch %v: got %d, want 400", b, rec.Code)
		}
	}

	rec = h.do(t, http.MethodPatch, "/profile", map[string]any{
		"expected_salary":    nil,
		"notice_period_days": nil,
	})
	p = decodeProfile(t, rec)
	if p.ExpectedSalary != nil || p.NoticePeriodDays != nil {
		t.Fatalf("nullable fields were not cleared: %+v", p)
	}
}

// AC-CV-5: confirm-before-apply gate. Arm is refused until the user confirms;
// editing after confirming re-locks the gate.
func TestAC_CV_5_ConfirmBeforeApplyGate(t *testing.T) {
	h := newHarness()

	// Fresh profile: unconfirmed, arm and confirmation refused.
	if rec := h.do(t, http.MethodGet, "/profile/arm", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("arm before confirm: got %d, want 403", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "/profile/confirm", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("confirm incomplete: got %d, want 400", rec.Code)
	}

	// Add the minimum complete profile, then confirm.
	h.do(t, http.MethodPatch, "/profile", map[string]any{
		"full_name":           "Dina",
		"email":               "dina@example.com",
		"phone":               "+628100000000",
		"education":           []map[string]any{{"institution": "UI"}},
		"skills":              []string{"Go"},
		"work_authorization":  "WNI",
		"preferred_locations": []string{"Jakarta"},
		"employment_type":     "full_time",
	})
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

func TestAC_CV_6_CompletenessScoreAndPrompts(t *testing.T) {
	p := Profile{
		FullName: "Dina", Email: "dina@example.com", Phone: "+62812",
		Education: json.RawMessage(`[{"institution":"UI"}]`),
		Skills:    json.RawMessage(`["Go"]`), WorkAuthorization: "WNI",
		PreferredLocations: json.RawMessage(`["Jakarta"]`), EmploymentType: "full_time",
	}
	result := CalculateCompleteness(p)
	if result.Score != 80 || result.Complete {
		t.Fatalf("completeness = %+v, want 80%% and incomplete", result)
	}
	if len(result.Missing) != 2 || result.Missing[0] != "work_history" || result.Missing[1] != "summary" {
		t.Fatalf("missing = %v", result.Missing)
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

func TestCanonicalFillProfileUsesStoredFieldsOnly(t *testing.T) {
	salary := int64(12000000)
	p := Profile{
		FullName:           "Sri Wahyuni",
		Email:              "sri@example.com",
		Phone:              "+628123456789",
		LinkedInURL:        "https://www.linkedin.com/in/sri",
		GitHubURL:          "https://github.com/sri",
		PortfolioURL:       "https://sri.example.com",
		Address:            "Jl. Sudirman 1",
		City:               "Jakarta Selatan",
		Summary:            "Software engineer",
		CurrentEmployer:    "Acme",
		CurrentTitle:       "Engineer",
		HighestEducation:   "S.Kom",
		Education:          json.RawMessage(`[{"institution":"UI","degree":"S.Kom","field":"Informatics","end_year":"2024"}]`),
		WorkHistory:        json.RawMessage(`[{"company":"Acme","title":"Engineer","end_date":""}]`),
		PreferredLocations: json.RawMessage(`["Bandung"]`),
		ExpectedSalary:     &salary,
		Confirmed:          true,
	}
	out := canonicalFillProfile(p)
	if out["first_name"] != "Sri" || out["last_name"] != "Wahyuni" ||
		out["linkedin_url"] != "https://www.linkedin.com/in/sri" ||
		out["github_url"] != "https://github.com/sri" ||
		out["portfolio_url"] != "https://sri.example.com" ||
		out["address"] != "Jl. Sudirman 1" || out["city"] != "Jakarta Selatan" ||
		out["summary"] != "Software engineer" || out["current_company"] != "Acme" ||
		out["current_title"] != "Engineer" || out["highest_education"] != "S.Kom" ||
		out["university"] != "UI" ||
		out["major"] != "Informatics" ||
		out["graduation_year"] != "2024" || out["expected_salary"] != "12000000" {
		t.Fatalf("unexpected canonical mapping: %#v", out)
	}
}

func TestProfileURLValidation(t *testing.T) {
	h := newHarness()
	for _, field := range []string{"linkedin_url", "github_url", "portfolio_url"} {
		t.Run(field, func(t *testing.T) {
			if rec := h.do(t, http.MethodPatch, "/profile", map[string]any{field: "not a URL"}); rec.Code != http.StatusBadRequest {
				t.Fatalf("invalid %s: got %d, want 400", field, rec.Code)
			}
			if rec := h.do(t, http.MethodPatch, "/profile", map[string]any{field: "https://example.com/me"}); rec.Code != http.StatusOK {
				t.Fatalf("valid %s: got %d, want 200", field, rec.Code)
			}
		})
	}
}

type fillCVSource struct {
	file  cv.File
	data  []byte
	calls *int
}

func (f fillCVSource) FilesByUser(_ context.Context, userID string) ([]cv.File, error) {
	if userID != f.file.UserID {
		return nil, cv.ErrNotFound
	}
	return []cv.File{f.file}, nil
}

func (f fillCVSource) Content(_ context.Context, userID, id string) (cv.File, []byte, error) {
	if f.calls != nil {
		*f.calls = *f.calls + 1
	}
	if userID != f.file.UserID || id != f.file.ID {
		return cv.File{}, nil, cv.ErrNotFound
	}
	return f.file, f.data, nil
}

func TestFillSnapshotIsOwnedAndOneShotData(t *testing.T) {
	repo := NewMemoryRepo()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	salary := int64(12000000)
	_, err := repo.Save(context.Background(), Profile{
		UserID: "user-1", FullName: "Sri Wahyuni", Email: "sri@example.com",
		Phone: "+628123456789", Education: json.RawMessage(`[{"institution":"UI"}]`),
		LinkedInURL: "https://linkedin.com/in/sri", GitHubURL: "https://github.com/sri",
		PortfolioURL: "https://sri.example.com", Address: "Jl. Sudirman 1",
		City: "Jakarta", Summary: "Software engineer", CurrentEmployer: "Acme",
		CurrentTitle: "Engineer", HighestEducation: "S.Kom",
		WorkHistory: json.RawMessage(`[]`), Skills: json.RawMessage(`["Go"]`),
		PreferredLocations: json.RawMessage(`["Jakarta"]`), WorkAuthorization: "WNI",
		EmploymentType: "full_time", ExpectedSalary: &salary, Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	const cvID = "11111111-1111-4111-8111-111111111111"
	file := fillCVSource{
		file: cv.File{ID: cvID, UserID: "user-1", Filename: "resume.pdf", ContentType: "application/pdf"},
		data: []byte("%PDF-real"),
	}
	svc := NewService(repo, func() time.Time { return now }).WithCVSource(file)
	handler := withUser(svc.Routes(), auth.User{ID: "user-1", Verified: true})
	req := httptest.NewRequest(http.MethodGet, "/profile/fill?cv_id="+cvID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" ||
		!bytes.Contains(rec.Body.Bytes(), []byte("JVBERi1yZWFs")) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"linkedin_url":"https://linkedin.com/in/sri"`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"current_company":"Acme"`)) {
		t.Fatalf("snapshot status/body = %d/%s", rec.Code, rec.Body.String())
	}

	for _, id := range []string{
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
	} {
		req = httptest.NewRequest(http.MethodGet, "/profile/fill?cv_id="+id, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("unowned or missing cv status = %d, want 404", rec.Code)
		}
	}
}

func TestFillSnapshotRejectsMalformedCVIDBeforeLookup(t *testing.T) {
	repo := NewMemoryRepo()
	svc := NewService(repo, time.Now).WithCVSource(fillCVSource{
		file: cv.File{ID: "11111111-1111-4111-8111-111111111111", UserID: "user-1"},
	})
	handler := withUser(svc.Routes(), auth.User{ID: "user-1", Verified: true})
	req := httptest.NewRequest(http.MethodGet, "/profile/fill?cv_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed cv id status = %d, want 400", rec.Code)
	}
}
