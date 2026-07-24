// Package api wires the Auto Applier HTTP routes.
//
// M0 scope: a health-check endpoint only. Per the Prime Directive, no route in
// this service ever submits a job application to a third-party portal.
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse is the JSON body returned by the health-check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// now is injectable so tests can assert deterministic output.
var now = time.Now

// Health responds with a small JSON payload indicating the service is up.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status: "ok",
		Time:   now().UTC().Format(time.RFC3339),
	})
}

// Mount is an additional route group to attach to the router at Pattern.
type Mount struct {
	Pattern string
	Handler http.Handler
}

// NewRouter builds the HTTP handler for the API, optionally mounting the auth
// service plus any extra route groups. Passing nil for authHandler mounts only
// the health endpoint.
func NewRouter(authHandler http.Handler, mounts ...Mount) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Health)
	if authHandler != nil {
		mux.Handle("/auth/", authHandler)
	}
	for _, m := range mounts {
		if m.Handler != nil && m.Pattern != "" {
			mux.Handle(m.Pattern, m.Handler)
		}
	}
	return mux
}
