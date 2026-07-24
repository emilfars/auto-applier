package safety

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// AC-SAFE-3 / Prime Directive: the backend must never submit a job application
// to a third-party portal. Submission is exclusively the human user's action in
// their own browser. This is an allowlist-free static guard over the backend
// source: it fails if any code appears to proxy/perform an external submission,
// or if any HTTP route is registered that looks like a submission proxy.
//
// It mirrors the extension-side AC-SAFE-1 scan on the client, closing the
// Prime-Directive loop on the server.

// backendRoot returns the /backend module root relative to this test file
// (backend/internal/safety/ -> backend/).
func backendRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// goSources walks the backend module and returns non-test .go files.
func goSources(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "testdata" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// forbidden identifiers/APIs that would indicate submitting an application to an
// external portal. Matched case-insensitively against source with whitespace and
// underscores stripped, so "submit application", "submit_application" and
// "SubmitApplication" all trip.
var forbidden = []struct {
	needle string
	why    string
}{
	{"submitapplication", "submitting an application"},
	{"applicationsubmit", "submitting an application"},
	{"submittoportal", "submitting to an external portal"},
	{"portalsubmit", "submitting to an external portal"},
	{"submitproxy", "proxying a submission"},
	{"proxysubmit", "proxying a submission"},
	{"autosubmit", "auto-submitting a form"},
	{"headlesssubmit", "headless submission"},
	{"clickapply", "programmatically clicking apply"},
	{"pressapply", "programmatically pressing apply"},
}

// normalize lowercases and removes spaces/underscores so multi-word phrasings
// of the same intent are caught.
func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func TestNoSubmissionProxyInBackendSource(t *testing.T) {
	root := backendRoot(t)
	files := goSources(t, root)
	if len(files) < 5 {
		t.Fatalf("expected to scan a non-trivial number of Go files, got %d", len(files))
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		norm := normalize(string(raw))
		for _, fb := range forbidden {
			if strings.Contains(norm, fb.needle) {
				rel, _ := filepath.Rel(root, f)
				t.Errorf("AC-SAFE-3 violation in %s: code suggests %s (matched %q)", rel, fb.why, fb.needle)
			}
		}
	}
}

// routeReg matches Go 1.22 mux registrations: HandleFunc("METHOD /path", ...),
// Handle("METHOD /path", ...), Mount("/path", ...).
var routeReg = regexp.MustCompile(`(?:HandleFunc|Handle|Mount)\(\s*"([^"]+)"`)

func TestNoSubmitProxyRouteRegistered(t *testing.T) {
	root := backendRoot(t)
	files := goSources(t, root)

	var routes []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range routeReg.FindAllStringSubmatch(string(raw), -1) {
			routes = append(routes, m[1])
		}
	}

	if len(routes) < 3 {
		t.Fatalf("expected to discover the API routes, found %d", len(routes))
	}

	for _, pat := range routes {
		// A route whose path denotes submitting an application would be a
		// submission proxy — forbidden regardless of implementation.
		if strings.Contains(strings.ToLower(pat), "submit") {
			t.Errorf("AC-SAFE-3 violation: submit-proxy-looking route registered: %q", pat)
		}
	}
}
