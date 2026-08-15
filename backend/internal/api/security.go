package api

import (
	"fmt"
	"net/http"
	"strings"
)

// SecurityConfig controls the transport-security middleware.
type SecurityConfig struct {
	// EnforceHTTPS rejects plain-HTTP requests and sets HSTS on secure ones.
	// Off by default so local/dev runs over http keep working; production sets
	// ENFORCE_HTTPS=true (AC-NFR-SEC).
	EnforceHTTPS bool
	// TrustForwardedProto allows a trusted reverse proxy to report the original
	// client scheme. Leave false when the API is directly reachable.
	TrustForwardedProto bool
	// HSTSMaxAge is the Strict-Transport-Security max-age in seconds. When <= 0
	// a two-year default is used.
	HSTSMaxAge int
}

const defaultHSTSMaxAge = 63072000 // 2 years, in seconds

// RequireHTTPS wraps next so that, when enforcement is enabled, requests that
// did not arrive over TLS are rejected and secure requests carry an HSTS
// header. When enforcement is disabled the handler is returned unchanged.
//
// The service runs behind a TLS-terminating proxy, so the original client
// scheme is read from X-Forwarded-Proto (set by that proxy) in addition to a
// direct TLS connection.
func RequireHTTPS(next http.Handler, cfg SecurityConfig) http.Handler {
	if !cfg.EnforceHTTPS {
		return next
	}
	maxAge := cfg.HSTSMaxAge
	if maxAge <= 0 {
		maxAge = defaultHSTSMaxAge
	}
	hsts := fmt.Sprintf("max-age=%d; includeSubDomains", maxAge)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requestIsSecure(r, cfg.TrustForwardedProto) {
			http.Error(w, "HTTPS is required", http.StatusForbidden)
			return
		}
		w.Header().Set("Strict-Transport-Security", hsts)
		next.ServeHTTP(w, r)
	})
}

// requestIsSecure reports whether the request reached the edge over HTTPS,
// either via a direct TLS connection or the proxy-supplied X-Forwarded-Proto.
func requestIsSecure(r *http.Request, trustForwardedProto bool) bool {
	if r.TLS != nil {
		return true
	}
	if !trustForwardedProto {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	// XFP can be a comma-separated list ("https, http"); the client-facing
	// scheme is the first entry.
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}
