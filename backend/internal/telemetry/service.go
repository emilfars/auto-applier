// Package telemetry accepts privacy-safe fill correction metadata.
package telemetry

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Service exposes the fill-correction ingestion boundary. It deliberately does
// not persist or log event values.
type Service struct{}

// NewService returns the metadata-only telemetry handler.
func NewService() *Service { return &Service{} }

type correction struct {
	Type     string `json:"type"`
	PortalID string `json:"portalId"`
	Version  string `json:"version"`
	Key      string `json:"key"`
	Selector string `json:"selector"`
	At       int64  `json:"at"`
}

// Routes returns the telemetry endpoint.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /telemetry/fill-correction", s.handleCorrection)
	return mux
}

func (s *Service) handleCorrection(w http.ResponseWriter, r *http.Request) {
	var event correction
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil ||
		event.Type != "fill_correction" ||
		!safeMeta(event.PortalID, 64) ||
		!safeMeta(event.Version, 32) ||
		!safeMeta(event.Key, 64) ||
		!safeMeta(event.Selector, 512) ||
		event.At <= 0 {
		http.Error(w, "invalid telemetry", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func safeMeta(value string, max int) bool {
	return value != "" && len(value) <= max && !strings.ContainsAny(value, "\r\n")
}
