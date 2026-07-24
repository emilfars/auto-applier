package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type harness struct {
	svc     *Service
	repo    *MemoryRepo
	handler http.Handler
	now     *time.Time
}

func newHarness() *harness {
	repo := NewMemoryRepo()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	h := &harness{repo: repo, now: &now}
	h.svc = NewService(repo, Config{
		SessionTTL:       time.Hour,
		ResetTTL:         time.Hour,
		LoginMaxAttempts: 5,
		LoginWindow:      15 * time.Minute,
		Now:              func() time.Time { return *h.now },
	})
	h.handler = h.svc.Routes()
	return h
}

func (h *harness) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("decode response: %v (%s)", err, rec.Body.String())
		}
	}
	return m
}

// registerVerifiedUser registers a user and marks them verified, returning id.
func (h *harness) registerVerifiedUser(t *testing.T, email, password string) string {
	t.Helper()
	rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: email, Password: password})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d, want 201", rec.Code)
	}
	id, _ := decodeBody(t, rec)["id"].(string)
	tok, ok := h.repo.VerificationToken(id)
	if !ok {
		t.Fatal("no verification token issued")
	}
	if rec := h.do(t, http.MethodPost, "/auth/verify", "", tokenReq{Token: tok}); rec.Code != http.StatusOK {
		t.Fatalf("verify: got %d, want 200", rec.Code)
	}
	return id
}

func (h *harness) login(t *testing.T, email, password string) (int, string) {
	t.Helper()
	rec := h.do(t, http.MethodPost, "/auth/login", "", registerReq{Email: email, Password: password})
	tok, _ := decodeBody(t, rec)["token"].(string)
	return rec.Code, tok
}

// AC-AUTH-1: register creates an unverified user and issues a verification token.
func TestAC_AUTH_1_RegisterCreatesUnverified(t *testing.T) {
	h := newHarness()
	rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "dina@example.com", Password: "password123"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["verified"] != false {
		t.Errorf("verified = %v, want false", body["verified"])
	}
	id := body["id"].(string)
	u, err := h.repo.UserByID(context.Background(), id)
	if err != nil || u.Verified {
		t.Errorf("user not persisted unverified: %+v err=%v", u, err)
	}
	if _, ok := h.repo.VerificationToken(id); !ok {
		t.Error("verification token not issued")
	}
	// password is not stored in plaintext
	if u.PasswordHash == "password123" || !strings.HasPrefix(u.PasswordHash, "pbkdf2_sha256$") {
		t.Errorf("password not hashed: %q", u.PasswordHash)
	}
}

func TestAC_AUTH_1_RegisterValidationAndDuplicate(t *testing.T) {
	h := newHarness()
	if rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "bad", Password: "password123"}); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid email: got %d, want 400", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "a@b.co", Password: "short"}); rec.Code != http.StatusBadRequest {
		t.Errorf("short password: got %d, want 400", rec.Code)
	}
	h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "dup@b.co", Password: "password123"})
	if rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "dup@b.co", Password: "password123"}); rec.Code != http.StatusConflict {
		t.Errorf("duplicate: got %d, want 409", rec.Code)
	}
}

// AC-AUTH-1b: unverified user is blocked (403) from gated routes; allowed (200)
// after verification.
func TestAC_AUTH_1b_GatedRouteVerificationGate(t *testing.T) {
	h := newHarness()
	rec := h.do(t, http.MethodPost, "/auth/register", "", registerReq{Email: "riz@example.com", Password: "password123"})
	id := decodeBody(t, rec)["id"].(string)

	_, token := h.login(t, "riz@example.com", "password123")
	if token == "" {
		t.Fatal("login before verify should still issue a session")
	}
	if rec := h.do(t, http.MethodGet, "/auth/me", token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("pre-verify gated route: got %d, want 403", rec.Code)
	}

	tok, _ := h.repo.VerificationToken(id)
	h.do(t, http.MethodPost, "/auth/verify", "", tokenReq{Token: tok})

	if rec := h.do(t, http.MethodGet, "/auth/me", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("post-verify gated route: got %d, want 200", rec.Code)
	}
}

func TestGatedRouteNoSession(t *testing.T) {
	h := newHarness()
	if rec := h.do(t, http.MethodGet, "/auth/me", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: got %d, want 401", rec.Code)
	}
	if rec := h.do(t, http.MethodGet, "/auth/me", "bogus-token", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad session: got %d, want 401", rec.Code)
	}
}

// AC-AUTH-4: login issues a session; logout invalidates it; expiry enforced.
func TestAC_AUTH_4_LoginLogoutAndExpiry(t *testing.T) {
	h := newHarness()
	h.registerVerifiedUser(t, "maya@example.com", "password123")

	code, token := h.login(t, "maya@example.com", "password123")
	if code != http.StatusOK || token == "" {
		t.Fatalf("login: got %d token=%q", code, token)
	}
	if rec := h.do(t, http.MethodGet, "/auth/me", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("me after login: got %d, want 200", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "/auth/logout", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("logout: got %d, want 200", rec.Code)
	}
	if rec := h.do(t, http.MethodGet, "/auth/me", token, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: got %d, want 401", rec.Code)
	}

	// expiry
	_, token2 := h.login(t, "maya@example.com", "password123")
	*h.now = h.now.Add(2 * time.Hour)
	if rec := h.do(t, http.MethodGet, "/auth/me", token2, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after expiry: got %d, want 401", rec.Code)
	}
}

func TestAC_AUTH_4_LoginWrongPassword(t *testing.T) {
	h := newHarness()
	h.registerVerifiedUser(t, "z@example.com", "password123")
	if code, _ := h.login(t, "z@example.com", "wrongpass1"); code != http.StatusUnauthorized {
		t.Errorf("wrong password: got %d, want 401", code)
	}
	if code, _ := h.login(t, "missing@example.com", "password123"); code != http.StatusUnauthorized {
		t.Errorf("unknown user: got %d, want 401", code)
	}
}

// AC-AUTH-4b: auth endpoints are rate-limited after N failures.
func TestAC_AUTH_4b_LoginRateLimited(t *testing.T) {
	h := newHarness()
	h.registerVerifiedUser(t, "rl@example.com", "password123")
	for i := 0; i < 5; i++ {
		if code, _ := h.login(t, "rl@example.com", "wrongpass1"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, code)
		}
	}
	if code, _ := h.login(t, "rl@example.com", "password123"); code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: got %d, want 429", code)
	}
	// window reset unblocks
	*h.now = h.now.Add(16 * time.Minute)
	if code, _ := h.login(t, "rl@example.com", "password123"); code != http.StatusOK {
		t.Fatalf("after window reset: got %d, want 200", code)
	}
}

// AC-AUTH-3: password reset request → token → set new password → old rejected.
func TestAC_AUTH_3_PasswordReset(t *testing.T) {
	h := newHarness()
	id := h.registerVerifiedUser(t, "reset@example.com", "password123")

	if rec := h.do(t, http.MethodPost, "/auth/password-reset/request", "", resetReq{Email: "reset@example.com"}); rec.Code != http.StatusOK {
		t.Fatalf("reset request: got %d, want 200", rec.Code)
	}
	// enumeration-safe: unknown email also 200
	if rec := h.do(t, http.MethodPost, "/auth/password-reset/request", "", resetReq{Email: "nobody@example.com"}); rec.Code != http.StatusOK {
		t.Fatalf("reset request unknown: got %d, want 200", rec.Code)
	}

	tok, ok := h.repo.ResetToken(id)
	if !ok {
		t.Fatal("no reset token issued")
	}
	if rec := h.do(t, http.MethodPost, "/auth/password-reset/confirm", "", resetConfirmReq{Token: tok, NewPassword: "newpassword456"}); rec.Code != http.StatusOK {
		t.Fatalf("reset confirm: got %d, want 200", rec.Code)
	}

	if code, _ := h.login(t, "reset@example.com", "password123"); code != http.StatusUnauthorized {
		t.Errorf("old password after reset: got %d, want 401", code)
	}
	if code, _ := h.login(t, "reset@example.com", "newpassword456"); code != http.StatusOK {
		t.Errorf("new password after reset: got %d, want 200", code)
	}
}

func TestPasswordResetInvalidToken(t *testing.T) {
	h := newHarness()
	if rec := h.do(t, http.MethodPost, "/auth/password-reset/confirm", "", resetConfirmReq{Token: "bogus", NewPassword: "newpassword456"}); rec.Code != http.StatusBadRequest {
		t.Errorf("bogus reset token: got %d, want 400", rec.Code)
	}
}

func TestPasswordResetExpired(t *testing.T) {
	h := newHarness()
	id := h.registerVerifiedUser(t, "exp@example.com", "password123")
	h.do(t, http.MethodPost, "/auth/password-reset/request", "", resetReq{Email: "exp@example.com"})
	tok, _ := h.repo.ResetToken(id)
	*h.now = h.now.Add(2 * time.Hour) // past ResetTTL
	if rec := h.do(t, http.MethodPost, "/auth/password-reset/confirm", "", resetConfirmReq{Token: tok, NewPassword: "newpassword456"}); rec.Code != http.StatusBadRequest {
		t.Errorf("expired reset token: got %d, want 400", rec.Code)
	}
}
