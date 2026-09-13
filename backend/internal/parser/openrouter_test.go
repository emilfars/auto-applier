package parser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenRouterExtract(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		content := "Here is the JSON:\n```json\n" +
			`{"name":"Jane Doe","email":"jane@example.com","phone":"+62 811","education":[],"work_history":[],"skills":["Go","SQL"]}` +
			"\n```"
		resp := map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"role": "assistant", "content": content},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	eng, err := NewOpenRouter(OpenRouterConfig{
		BaseURL: srv.URL,
		APIKey:  "secret-key",
		Models:  []string{"model-a", "model-b"},
	}, srv.Client())
	if err != nil {
		t.Fatalf("NewOpenRouter: %v", err)
	}
	out, err := eng.Extract(context.Background(), "Jane Doe, Data Analyst")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if out.Name != "Jane Doe" || out.Email != "jane@example.com" {
		t.Fatalf("unexpected extraction: %+v", out)
	}
	if len(out.Skills) != 2 || out.Skills[0] != "Go" {
		t.Fatalf("unexpected skills: %+v", out.Skills)
	}

	if gotAuth != "Bearer secret-key" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["model"] != "model-a" {
		t.Fatalf("model = %v", gotBody["model"])
	}
	if gotBody["response_format"] == nil {
		t.Fatal("response_format missing")
	}
	models, ok := gotBody["models"].([]any)
	if !ok || len(models) != 2 {
		t.Fatalf("models = %v", gotBody["models"])
	}
	msgs, _ := gotBody["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages = %v", gotBody["messages"])
	}
	msg, _ := msgs[0].(map[string]any)
	if content, _ := msg["content"].(string); !strings.Contains(content, "Jane Doe, Data Analyst") {
		t.Fatalf("prompt does not carry resume text")
	}
}

func TestOpenRouterErrorDoesNotLeakBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "SECRET-RESUME-CONTENTS")
	}))
	defer srv.Close()

	eng, err := NewOpenRouter(OpenRouterConfig{BaseURL: srv.URL, APIKey: "k", Models: []string{"m"}}, srv.Client())
	if err != nil {
		t.Fatalf("NewOpenRouter: %v", err)
	}
	_, err = eng.Extract(context.Background(), "my private cv")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "provider status 500") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaked provider body: %v", err)
	}
}

func TestNewOpenRouterValidation(t *testing.T) {
	cases := map[string]OpenRouterConfig{
		"missing key":    {BaseURL: "https://openrouter.ai/api/v1", Models: []string{"m"}},
		"missing models": {BaseURL: "https://openrouter.ai/api/v1", APIKey: "k"},
		"insecure host":  {BaseURL: "http://openrouter.example.com/api/v1", APIKey: "k", Models: []string{"m"}},
	}
	for name, cfg := range cases {
		if _, err := NewOpenRouter(cfg, nil); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestFirstJSONObject(t *testing.T) {
	got, ok := firstJSONObject("```json\n{\"a\":1}\n```")
	if !ok || got != `{"a":1}` {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if _, ok := firstJSONObject("no json here"); ok {
		t.Fatal("expected no JSON")
	}
}
