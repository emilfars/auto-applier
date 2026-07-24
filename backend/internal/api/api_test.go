package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   bool
	}{
		{name: "healthz ok", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: true},
		{name: "unknown route 404", method: http.MethodGet, path: "/nope", wantStatus: http.StatusNotFound},
		{name: "wrong method 405", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed},
	}

	router := NewRouter()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if !tc.wantBody {
				return
			}

			var got HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.Status != "ok" {
				t.Errorf("status field = %q, want %q", got.Status, "ok")
			}
			if got.Time == "" {
				t.Error("time field is empty")
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q, want application/json", ct)
			}
		})
	}
}
