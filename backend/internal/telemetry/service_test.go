package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFillCorrectionMetadataOnly(t *testing.T) {
	svc := NewService()
	tests := []struct {
		name string
		body string
		code int
	}{
		{"valid", `{"type":"fill_correction","portalId":"lever","version":"1.0.0","key":"email","selector":"input[type=email]","at":1}`, http.StatusNoContent},
		{"reject values", `{"type":"fill_correction","portalId":"lever","version":"1.0.0","key":"email","selector":"input","at":1,"value":"pii"}`, http.StatusBadRequest},
		{"reject invalid metadata", `{"type":"fill_correction","portalId":"","version":"1.0.0","key":"email","selector":"input","at":1}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/telemetry/fill-correction", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			svc.Routes().ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d", rec.Code, tc.code)
			}
		})
	}
}
