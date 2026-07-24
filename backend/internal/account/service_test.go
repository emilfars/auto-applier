package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
	"github.com/auto-applier/backend/internal/profile"
	"github.com/auto-applier/backend/internal/storage"
)

type fixture struct {
	svc       *Service
	authRepo  *auth.MemoryRepo
	profRepo  *profile.MemoryRepo
	cvRepo    *cv.MemoryRepo
	store     *storage.MemoryStore
	userID    string
	objectKey string
}

// newFixture builds a verified user with a saved profile and one stored CV file
// (object + metadata), wired through the real in-memory repos.
func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()

	authRepo := auth.NewMemoryRepo()
	u, err := authRepo.CreateUser(ctx, "erase@example.com", "pbkdf2_sha256$hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := authRepo.SetConsent(ctx, u.ID, time.Now()); err != nil {
		t.Fatalf("set consent: %v", err)
	}
	if err := authRepo.SetVerified(ctx, u.ID); err != nil {
		t.Fatalf("set verified: %v", err)
	}

	profRepo := profile.NewMemoryRepo()
	if _, err := profRepo.Save(ctx, profile.Profile{
		UserID:      u.ID,
		FullName:    "Dina Sari",
		Phone:       "+628123456789",
		Education:   json.RawMessage("[]"),
		WorkHistory: json.RawMessage("[]"),
		Skills:      json.RawMessage("[]"),
	}); err != nil {
		t.Fatalf("save profile: %v", err)
	}

	store := storage.NewMemoryStore()
	cvRepo := cv.NewMemoryRepo()
	cvSvc := cv.NewService(cvRepo, store, time.Now)
	const objectKey = "cv/erase/object-1"
	if err := store.Put(ctx, objectKey, []byte("ciphertext-bytes")); err != nil {
		t.Fatalf("put object: %v", err)
	}
	if _, err := cvRepo.CreateFile(ctx, cv.File{
		UserID:      u.ID,
		ObjectKey:   objectKey,
		Filename:    "resume.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1234,
	}); err != nil {
		t.Fatalf("create file: %v", err)
	}

	return fixture{
		svc:       NewService(authRepo, profRepo, cvSvc),
		authRepo:  authRepo,
		profRepo:  profRepo,
		cvRepo:    cvRepo,
		store:     store,
		userID:    u.ID,
		objectKey: objectKey,
	}
}

// do serves a request against the account routes with the fixture's user
// injected into context (as RequireVerified would in production).
func (f fixture) do(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	u, err := f.authRepo.UserByID(context.Background(), f.userID)
	if err != nil {
		// The account may already be deleted; fall back to a minimal identity.
		u = auth.User{ID: f.userID}
	}
	req := httptest.NewRequest(method, target, nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	rec := httptest.NewRecorder()
	f.svc.Routes().ServeHTTP(rec, req)
	return rec
}

// AC-AUTH-5: data export returns the complete user data set.
func TestAC_AUTH_5_ExportReturnsCompleteData(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/account/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("export: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var out exportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if out.Account.Email != "erase@example.com" {
		t.Errorf("account.email = %q, want erase@example.com", out.Account.Email)
	}
	if out.Account.ConsentAt == nil {
		t.Error("export missing consent_at")
	}
	if out.Profile == nil || out.Profile.FullName != "Dina Sari" {
		t.Errorf("export profile = %+v, want full_name Dina Sari", out.Profile)
	}
	if len(out.CVFiles) != 1 || out.CVFiles[0].Filename != "resume.pdf" {
		t.Errorf("export cv_files = %+v, want one resume.pdf", out.CVFiles)
	}
}

// AC-AUTH-5: account deletion removes all PII — account, profile, CV metadata,
// and the encrypted CV object in storage.
func TestAC_AUTH_5_DeleteRemovesAllPII(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodDelete, "/account")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}

	ctx := context.Background()
	if _, err := f.authRepo.UserByID(ctx, f.userID); err == nil {
		t.Error("user still exists after deletion")
	}
	if _, err := f.profRepo.Get(ctx, f.userID); err == nil {
		t.Error("profile still exists after deletion")
	}
	if files, _ := f.cvRepo.FilesByUser(ctx, f.userID); len(files) != 0 {
		t.Errorf("cv metadata not deleted: %d files remain", len(files))
	}
	if _, err := f.store.Get(ctx, f.objectKey); err == nil {
		t.Error("encrypted CV object still present in storage after deletion")
	}
}

// AC-AUTH-5: the endpoints refuse unauthenticated callers (no user in context).
func TestAC_AUTH_5_RequiresAuthenticatedUser(t *testing.T) {
	f := newFixture(t)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/account/export"},
		{http.MethodDelete, "/account"},
	} {
		req := httptest.NewRequest(tc.method, tc.target, nil)
		rec := httptest.NewRecorder()
		f.svc.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", tc.method, tc.target, rec.Code)
		}
	}
}
