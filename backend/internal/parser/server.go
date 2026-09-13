package parser

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Server serves the parser service HTTP API.
type Server struct {
	svc     *Service
	apiKey  string
	maxBody int64
	handler http.Handler
}

// NewServer builds the HTTP handler. When apiKey is non-empty, requests must
// present it as `Authorization: Bearer <apiKey>`.
func NewServer(svc *Service, apiKey string) *Server {
	s := &Server{svc: svc, apiKey: apiKey, maxBody: int64(svc.maxBytes)*2 + 4096}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /parse", s.handleParse)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	s.handler = mux
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleParse(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	var doc Document
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&doc); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	out, err := s.svc.Parse(r.Context(), doc)
	if err != nil {
		status, msg := statusForError(err)
		writeError(w, status, msg)
		return
	}
	writeJSON(w, http.StatusOK, Response{Data: out})
}

func (s *Server) authorized(r *http.Request) bool {
	if s.apiKey == "" {
		return true
	}
	token := bearerToken(r.Header.Get("Authorization"))
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.apiKey)) == 1
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// statusForError maps service errors to a status code and a client-safe message
// (never echoing document contents or provider details).
func statusForError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrTooLarge):
		return http.StatusRequestEntityTooLarge, "document too large"
	case errors.Is(err, ErrUnsupportedType):
		return http.StatusUnsupportedMediaType, "unsupported document type"
	case errors.Is(err, ErrEmptyText):
		return http.StatusUnprocessableEntity, "no extractable text in document"
	case errors.Is(err, ErrBadDocument):
		return http.StatusBadRequest, "malformed document"
	case errors.Is(err, ErrEngine):
		return http.StatusBadGateway, "resume parsing failed"
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
