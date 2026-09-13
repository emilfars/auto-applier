package parser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/auto-applier/backend/internal/cv"
)

// TestLiveStackHostedParserWithRealOpenRouterEngine exercises the complete
// production path in one pass: the real cv.HostedParser client posts the
// envelope to the real HTTP server, which runs the real DOCX/PDF extractors and
// the real OpenRouter engine against an httptest provider. It closes the gap
// left by the unit tests, which cover the server and the engine separately.
func TestLiveStackHostedParserWithRealOpenRouterEngine(t *testing.T) {
	const providerKey = "provider-test-key"
	const parserKey = "parser-test-key"

	var mu sync.Mutex
	var seen []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Errorf("provider got %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+providerKey {
			t.Errorf("provider authorization = %q", got)
		}
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		mu.Lock()
		if len(req.Messages) > 0 {
			seen = append(seen, req.Messages[0].Content)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"role": "assistant",
					"content": "```json\n" + `{"name":"Jane Doe","email":"JANE.DOE@example.com",` +
						`"phone":"+62 811 2233 4455","education":[{"institution":"Institut Teknologi Bandung",` +
						`"degree":"Bachelor of Science","field_of_study":"Computer Science","start_year":"2012",` +
						`"end_year":"2016"}],"work_history":[{"company":"Traveloka","title":"Data Analyst",` +
						`"start_date":"2017","end_date":"Present"}],"skills":["SQL","Python"]}` + "\n```",
				},
			}},
		})
	}))
	defer provider.Close()

	eng, err := NewOpenRouter(OpenRouterConfig{
		BaseURL: provider.URL,
		APIKey:  providerKey,
		Models:  []string{"mock/model"},
	}, provider.Client())
	if err != nil {
		t.Fatalf("NewOpenRouter: %v", err)
	}

	svc := NewService(DefaultExtractor{}, eng, 1<<20)
	srv := httptest.NewServer(NewServer(svc, parserKey))
	defer srv.Close()

	client := cv.NewHostedParser(srv.URL+"/parse", parserKey, srv.Client())

	want := cv.ParsedCV{
		FullName: "Jane Doe",
		Email:    "jane.doe@example.com",
		Phone:    "+6281122334455",
		Education: []cv.EducationEntry{{
			Institution: "Institut Teknologi Bandung",
			Degree:      "Bachelor of Science",
			Field:       "Computer Science",
			StartYear:   "2012",
			EndYear:     "2016",
		}},
		WorkHistory: []cv.WorkEntry{{
			Company:   "Traveloka",
			Title:     "Data Analyst",
			StartDate: "2017",
			EndDate:   "",
		}},
		Skills: []string{"Python", "SQL"},
	}

	docx := buildDocx(t, []string{"DOCXMARKER", "Jane Doe", "jane.doe@example.com"})
	gotDocx, err := client.Parse(context.Background(), docx, "resume.docx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	if err != nil {
		t.Fatalf("hosted parse docx: %v", err)
	}
	if !reflect.DeepEqual(gotDocx, want) {
		t.Fatalf("docx round-trip mismatch\n got:  %+v\n want: %+v", gotDocx, want)
	}

	pdf := buildPDF(t, "PDFMARKER Jane Doe jane.doe@example.com")
	gotPDF, err := client.Parse(context.Background(), pdf, "resume.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("hosted parse pdf: %v", err)
	}
	if !reflect.DeepEqual(gotPDF, want) {
		t.Fatalf("pdf round-trip mismatch\n got:  %+v\n want: %+v", gotPDF, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if !containsSubstring(seen, "DOCXMARKER") {
		t.Fatalf("provider never received DOCX text: %d calls", len(seen))
	}
	if !containsSubstring(seen, "PDFMARKER") {
		t.Fatalf("provider never received PDF text: %d calls", len(seen))
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
