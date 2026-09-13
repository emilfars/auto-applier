package parser

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func postParse(t *testing.T, srv *httptest.Server, apiKey string, doc Document) *http.Response {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/parse", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestServerRequiresAuth(t *testing.T) {
	svc := NewService(fakeExtractor{text: "resume"}, &fakeEngine{}, 1<<20)
	srv := httptest.NewServer(NewServer(svc, "secret"))
	defer srv.Close()

	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("data"))}
	if resp := postParse(t, srv, "", doc); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no key: status %d, want 401", resp.StatusCode)
	}
	if resp := postParse(t, srv, "wrong", doc); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong key: status %d, want 401", resp.StatusCode)
	}
	resp := postParse(t, srv, "secret", doc)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("correct key: status %d, want 200", resp.StatusCode)
	}
}

func TestServerRejectsBadJSON(t *testing.T) {
	svc := NewService(fakeExtractor{text: "resume"}, &fakeEngine{}, 1<<20)
	srv := httptest.NewServer(NewServer(svc, ""))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/parse", bytes.NewReader([]byte("{not json")))
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
}

func TestServerStatusMapping(t *testing.T) {
	cases := []struct {
		name   string
		svc    *Service
		body   []byte
		status int
	}{
		{
			name:   "unsupported media",
			svc:    NewService(fakeExtractor{err: ErrUnsupportedType}, &fakeEngine{}, 1<<20),
			body:   []byte("hello"),
			status: http.StatusUnsupportedMediaType,
		},
		{
			name:   "too large",
			svc:    NewService(fakeExtractor{text: "x"}, &fakeEngine{}, 4),
			body:   []byte("way too many bytes"),
			status: http.StatusRequestEntityTooLarge,
		},
		{
			name:   "empty text",
			svc:    NewService(fakeExtractor{text: "  "}, &fakeEngine{}, 1<<20),
			body:   []byte("hello"),
			status: http.StatusUnprocessableEntity,
		},
		{
			name:   "engine failure",
			svc:    NewService(fakeExtractor{text: "resume"}, &fakeEngine{err: ErrEngine}, 1<<20),
			body:   []byte("hello"),
			status: http.StatusBadGateway,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(NewServer(tc.svc, ""))
			defer srv.Close()
			doc := Document{DocumentB64: base64.StdEncoding.EncodeToString(tc.body)}
			resp := postParse(t, srv, "", doc)
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("status %d, want %d", resp.StatusCode, tc.status)
			}
		})
	}
}

func TestServerHappyPathEnvelope(t *testing.T) {
	eng := &fakeEngine{out: Extraction{Name: "Budi Santoso", Skills: []string{"Go"}}}
	svc := NewService(fakeExtractor{text: "resume"}, eng, 1<<20)
	srv := httptest.NewServer(NewServer(svc, ""))
	defer srv.Close()

	doc := Document{DocumentB64: base64.StdEncoding.EncodeToString([]byte("data"))}
	resp := postParse(t, srv, "", doc)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out struct {
		Data Extraction `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Data.Name != "Budi Santoso" {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}

func TestServerHealth(t *testing.T) {
	svc := NewService(fakeExtractor{text: "x"}, &fakeEngine{}, 1<<20)
	srv := httptest.NewServer(NewServer(svc, ""))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", resp.StatusCode)
	}
}
