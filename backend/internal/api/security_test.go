package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler records whether the wrapped handler was reached.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

// AC-NFR-SEC: with HTTPS enforced, a plain-HTTP request is rejected and never
// reaches the application handler.
func TestRequireHTTPS_RejectsPlainHTTP_AC_NFR_SEC(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
	}{
		{name: "no forwarded proto", headers: nil},
		{name: "forwarded http", headers: map[string]string{"X-Forwarded-Proto": "http"}},
		{name: "forwarded list http first", headers: map[string]string{"X-Forwarded-Proto": "http, https"}},
		{name: "untrusted forwarded https", headers: map[string]string{"X-Forwarded-Proto": "https"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			h := RequireHTTPS(okHandler(&reached), SecurityConfig{EnforceHTTPS: true})

			req := httptest.NewRequest(http.MethodGet, "http://api.example/profile", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("plain HTTP: got status %d, want %d", rec.Code, http.StatusForbidden)
			}
			if reached {
				t.Fatal("plain HTTP request reached the application handler")
			}
			if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
				t.Fatalf("HSTS should not be set on a rejected request, got %q", got)
			}
		})
	}
}

// AC-NFR-SEC: with HTTPS enforced, a secure request passes through and carries
// an HSTS header. Secure is detected via a direct TLS connection or the
// proxy-supplied X-Forwarded-Proto.
func TestRequireHTTPS_AllowsSecure_AC_NFR_SEC(t *testing.T) {
	cases := []struct {
		name    string
		withTLS bool
		proto   string
		trust   bool
	}{
		{name: "forwarded https", proto: "https", trust: true},
		{name: "forwarded list https first", proto: "https, http", trust: true},
		{name: "direct tls", withTLS: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			h := RequireHTTPS(okHandler(&reached), SecurityConfig{
				EnforceHTTPS:        true,
				TrustForwardedProto: tc.trust,
			})

			req := httptest.NewRequest(http.MethodGet, "http://api.example/profile", nil)
			if tc.withTLS {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("secure request: got status %d, want %d", rec.Code, http.StatusOK)
			}
			if !reached {
				t.Fatal("secure request did not reach the application handler")
			}
			if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
				t.Fatal("secure request is missing the Strict-Transport-Security header")
			}
		})
	}
}

// AC-NFR-SEC: enforcement is opt-in, so local/dev (default config) still serves
// plain HTTP without rejection or HSTS.
func TestRequireHTTPS_DisabledAllowsPlainHTTP_AC_NFR_SEC(t *testing.T) {
	reached := false
	h := RequireHTTPS(okHandler(&reached), SecurityConfig{EnforceHTTPS: false})

	req := httptest.NewRequest(http.MethodGet, "http://api.example/profile", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !reached {
		t.Fatalf("disabled enforcement should pass plain HTTP through, got status %d reached=%v", rec.Code, reached)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS should not be set when enforcement is disabled, got %q", got)
	}
}
