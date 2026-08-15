package auth

import (
	"context"
	"errors"
	"net/http"
)

// OIDCClaims are the identity claims returned by an OIDC provider after a
// successful authorization-code exchange.
type OIDCClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// OIDCProvider exchanges an authorization code for verified identity claims.
// Production uses Google; tests use a fake. This keeps the auth service free of
// any concrete OAuth client dependency.
type OIDCProvider interface {
	Exchange(ctx context.Context, code string) (OIDCClaims, error)
}

type oauthCallbackReq struct {
	Code    string `json:"code"`
	Consent bool   `json:"consent"`
}

// handleGoogleCallback exchanges an auth code, then links or creates a verified
// account and issues a session (AUTH-2). It never accepts an unverified email.
func (s *Service) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.ipAllowed(w, r) {
		return
	}
	var req oauthCallbackReq
	if !decode(w, r, &req) {
		return
	}
	if req.Code == "" {
		writeErr(w, http.StatusBadRequest, "missing code")
		return
	}
	claims, err := s.cfg.GoogleOIDC.Exchange(r.Context(), req.Code)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "oauth exchange failed")
		return
	}
	if claims.Email == "" || !claims.EmailVerified {
		writeErr(w, http.StatusUnauthorized, "email not present or unverified")
		return
	}

	u, err := s.repo.UserByEmail(r.Context(), claims.Email)
	if errors.Is(err, ErrNotFound) {
		if !req.Consent {
			writeErr(w, http.StatusBadRequest, "consent to data processing is required")
			return
		}
		// New OAuth account: no local password, email already verified by Google.
		u, err = s.repo.CreateUser(r.Context(), claims.Email, "")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "create user error")
			return
		}
		if err := s.repo.SetConsent(r.Context(), u.ID, s.cfg.Now()); err != nil {
			s.deleteNewUser(r.Context(), u.ID)
			writeErr(w, http.StatusInternalServerError, "consent error")
			return
		}
		if err := s.repo.SetVerified(r.Context(), u.ID); err != nil {
			s.deleteNewUser(r.Context(), u.ID)
			writeErr(w, http.StatusInternalServerError, "verify error")
			return
		}
		u.Verified = true
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "lookup error")
		return
	} else if u.ConsentAt.IsZero() {
		if !req.Consent {
			writeErr(w, http.StatusBadRequest, "consent to data processing is required")
			return
		}
		if err := s.repo.SetConsent(r.Context(), u.ID, s.cfg.Now()); err != nil {
			writeErr(w, http.StatusInternalServerError, "consent error")
			return
		}
	}
	if !u.Verified {
		// Existing local account, now proven via Google — mark verified.
		if err := s.repo.SetVerified(r.Context(), u.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "verify error")
			return
		}
	}

	_, err = s.startSession(r.Context(), w, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "authenticated", "id": u.ID})
}
