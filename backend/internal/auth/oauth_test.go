package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// fakeOIDC is a mocked OIDC provider for AUTH-2 tests.
type fakeOIDC struct {
	claims OIDCClaims
	err    error
}

func (f fakeOIDC) Exchange(_ context.Context, code string) (OIDCClaims, error) {
	if f.err != nil {
		return OIDCClaims{}, f.err
	}
	if code == "bad-code" {
		return OIDCClaims{}, errors.New("invalid code")
	}
	return f.claims, nil
}

func newOAuthHarness(provider OIDCProvider) *harness {
	repo := NewMemoryRepo()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	h := &harness{repo: repo, now: &now}
	h.svc = NewService(repo, Config{
		SessionTTL: time.Hour,
		Now:        func() time.Time { return *h.now },
		GoogleOIDC: provider,
	})
	h.handler = h.svc.Routes()
	return h
}

// AC-AUTH-2: OAuth callback creates a verified account and issues a session.
func TestAC_AUTH_2_OAuthCreatesAccount(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{claims: OIDCClaims{Subject: "g-1", Email: "oauth@example.com", EmailVerified: true}})

	rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "good-code", Consent: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("callback: got %d, want 200", rec.Code)
	}
	body := decodeBody(t, rec)
	token := cookieValue(rec, sessionCookie)
	if token == "" {
		t.Fatal("no session token issued")
	}
	if _, ok := body["token"]; ok {
		t.Fatal("session token exposed in response body")
	}
	// account exists and is verified
	u, err := h.repo.UserByEmail(context.Background(), "oauth@example.com")
	if err != nil || !u.Verified {
		t.Fatalf("oauth user not created verified: %+v err=%v", u, err)
	}
	// gated route works immediately (verified)
	if rec := h.do(t, http.MethodGet, "/auth/me", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("gated route with oauth session: got %d, want 200", rec.Code)
	}
}

// AC-AUTH-2: OAuth links to an existing account (same user id, no duplicate).
func TestAC_AUTH_2_OAuthLinksExistingAccount(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{claims: OIDCClaims{Subject: "g-2", Email: "dina@example.com", EmailVerified: true}})
	// pre-existing local account
	existing, err := h.repo.CreateUser(context.Background(), "dina@example.com", "pbkdf2_sha256$1$c2FsdA$aGFzaA")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "good-code", Consent: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("callback: got %d, want 200", rec.Code)
	}
	if id, _ := decodeBody(t, rec)["id"].(string); id != existing.ID {
		t.Fatalf("linked to wrong user: got %q, want %q", id, existing.ID)
	}
}

func TestAC_AUTH_2_OAuthRejectsUnverifiedEmail(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{claims: OIDCClaims{Subject: "g-3", Email: "sketchy@example.com", EmailVerified: false}})
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "good-code"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unverified email: got %d, want 401", rec.Code)
	}
}

func TestAC_AUTH_2_OAuthExchangeFailure(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{claims: OIDCClaims{Email: "x@example.com", EmailVerified: true}})
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "bad-code"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad code: got %d, want 401", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: ""}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty code: got %d, want 400", rec.Code)
	}
}

func TestAC_AUTH_2_OAuthNewAccountRequiresConsent(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{claims: OIDCClaims{Subject: "g-4", Email: "oauth-consent@example.com", EmailVerified: true}})
	rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "good-code"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oauth without consent: got %d, want 400", rec.Code)
	}
	if _, err := h.repo.UserByEmail(context.Background(), "oauth-consent@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("account created without consent: %v", err)
	}
}

func TestAC_AUTH_4b_OAuthRateLimited(t *testing.T) {
	h := newOAuthHarness(fakeOIDC{err: errors.New("exchange failed")})
	h.svc.ipLimiter = newRateLimiter(1, time.Minute)
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "bad"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("first oauth attempt: got %d, want 401", rec.Code)
	}
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "bad"}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second oauth attempt: got %d, want 429", rec.Code)
	}
}

// When no provider is configured, the OAuth route is not registered.
func TestOAuthRouteDisabledWithoutProvider(t *testing.T) {
	h := newHarness() // no GoogleOIDC
	if rec := h.do(t, http.MethodPost, "/auth/oauth/google/callback", "", oauthCallbackReq{Code: "x"}); rec.Code != http.StatusNotFound {
		t.Fatalf("oauth without provider: got %d, want 404", rec.Code)
	}
}
