package cv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestHostedParserMapsResponse checks the HTTP round-trip: the parser posts a
// JSON envelope and maps the API response into a normalized ParsedCV.
func TestHostedParserMapsResponse(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "id_response.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var gotReq hostedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p := NewHostedParser(srv.URL, "secret", srv.Client())
	got, err := p.Parse(context.Background(), []byte("%PDF-1.7 data"), "cv.pdf", contentTypePDF)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if gotReq.Filename != "cv.pdf" || gotReq.ContentType != contentTypePDF || gotReq.DocumentB64 == "" {
		t.Errorf("request envelope not populated: %+v", gotReq)
	}

	var want ParsedCV
	readJSON(t, "id_expected.json", &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hosted parse != fixture\n got:  %+v\n want: %+v", got, want)
	}
}

func TestHostedParserNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := NewHostedParser(srv.URL, "", srv.Client())
	if _, err := p.Parse(context.Background(), []byte("x"), "cv.pdf", contentTypePDF); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestHostedParserNoEndpoint(t *testing.T) {
	p := NewHostedParser("", "", nil)
	if _, err := p.Parse(context.Background(), []byte("x"), "cv.pdf", contentTypePDF); err != ErrParserUnavailable {
		t.Fatalf("err = %v, want ErrParserUnavailable", err)
	}
}

func TestDisabledParser(t *testing.T) {
	if _, err := NewDisabledParser().Parse(context.Background(), nil, "", ""); err != ErrParserUnavailable {
		t.Fatalf("err = %v, want ErrParserUnavailable", err)
	}
}
