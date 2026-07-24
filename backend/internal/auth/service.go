package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Config tunes the auth service. Zero values fall back to secure defaults.
type Config struct {
	SessionTTL       time.Duration
	ResetTTL         time.Duration
	LoginMaxAttempts int
	LoginWindow      time.Duration
	Now              func() time.Time
}

func (c Config) withDefaults() Config {
	if c.SessionTTL == 0 {
		c.SessionTTL = 24 * time.Hour
	}
	if c.ResetTTL == 0 {
		c.ResetTTL = time.Hour
	}
	if c.LoginMaxAttempts == 0 {
		c.LoginMaxAttempts = 5
	}
	if c.LoginWindow == 0 {
		c.LoginWindow = 15 * time.Minute
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Service exposes the auth HTTP API over a Repo.
type Service struct {
	repo    Repo
	cfg     Config
	limiter *rateLimiter
}

// NewService builds an auth service.
func NewService(repo Repo, cfg Config) *Service {
	cfg = cfg.withDefaults()
	return &Service{
		repo:    repo,
		cfg:     cfg,
		limiter: newRateLimiter(cfg.LoginMaxAttempts, cfg.LoginWindow),
	}
}

const sessionCookie = "session"

type ctxKey int

const userCtxKey ctxKey = iota

// UserFrom returns the authenticated user attached by RequireVerified.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

// Routes returns the auth HTTP handler.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/verify", s.handleVerify)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/password-reset/request", s.handleResetRequest)
	mux.HandleFunc("POST /auth/password-reset/confirm", s.handleResetConfirm)
	mux.Handle("GET /auth/me", s.RequireVerified(http.HandlerFunc(s.handleMe)))
	return mux
}

// --- handlers ---

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Service) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	if !validEmail(req.Email) || len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "invalid email or password too short (min 8)")
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	u, err := s.repo.CreateUser(r.Context(), req.Email, hash)
	if errors.Is(err, ErrEmailTaken) {
		writeErr(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create user error")
		return
	}
	token, err := NewToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	if err := s.repo.CreateVerification(r.Context(), u.ID, token); err != nil {
		writeErr(w, http.StatusInternalServerError, "verification error")
		return
	}
	// Production emails the token; it is never returned in the response.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": u.ID, "email": u.Email, "verified": u.Verified,
	})
}

type tokenReq struct {
	Token string `json:"token"`
}

func (s *Service) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req tokenReq
	if !decode(w, r, &req) {
		return
	}
	userID, err := s.repo.ConsumeVerification(r.Context(), req.Token)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid verification token")
		return
	}
	if err := s.repo.SetVerified(r.Context(), userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "verify error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	key := "login:" + strings.ToLower(req.Email)
	if !s.limiter.Allow(key, s.cfg.Now()) {
		writeErr(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	u, err := s.repo.UserByEmail(r.Context(), req.Email)
	if err != nil || !VerifyPassword(u.PasswordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := NewToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	sess := Session{Token: token, UserID: u.ID, ExpiresAt: s.cfg.Now().Add(s.cfg.SessionTTL)}
	if err := s.repo.CreateSession(r.Context(), sess); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}
	s.limiter.Reset(key)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := sessionToken(r); token != "" {
		_ = s.repo.DeleteSession(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id": u.ID, "email": u.Email, "verified": u.Verified,
	})
}

type resetReq struct {
	Email string `json:"email"`
}

func (s *Service) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if !decode(w, r, &req) {
		return
	}
	// Always 200 to avoid account enumeration.
	if u, err := s.repo.UserByEmail(r.Context(), req.Email); err == nil {
		if token, err := NewToken(); err == nil {
			_ = s.repo.CreateReset(r.Context(), u.ID, token, s.cfg.Now().Add(s.cfg.ResetTTL))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type resetConfirmReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (s *Service) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	var req resetConfirmReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 8 {
		writeErr(w, http.StatusBadRequest, "password too short (min 8)")
		return
	}
	userID, err := s.repo.ConsumeReset(r.Context(), req.Token, s.cfg.Now())
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	if err := s.repo.UpdatePassword(r.Context(), userID, hash); err != nil {
		writeErr(w, http.StatusInternalServerError, "update error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "password_updated"})
}

// RequireVerified guards routes: 401 without a valid session, 403 when the
// user's email is not verified.
func (s *Service) RequireVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		sess, err := s.repo.SessionByToken(r.Context(), token)
		if err != nil || s.cfg.Now().After(sess.ExpiresAt) {
			writeErr(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		u, err := s.repo.UserByID(r.Context(), sess.UserID)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid session")
			return
		}
		if !u.Verified {
			writeErr(w, http.StatusForbidden, "email not verified")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- helpers ---

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t\n")
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
