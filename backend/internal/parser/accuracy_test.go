package parser

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestACCV2EngineAccuracy checks the live OpenRouter engine against the labeled
// Indonesian and English résumés from cv/testdata (feeds the AC-CV-2 ≥90% field
// accuracy target). It is opt-in because it makes real, billable model calls:
//
//	RUN_PARSER_ACCURACY=1 OPENROUTER_API_KEY=... go test ./internal/parser -run AC_CV_2
func TestACCV2EngineAccuracy(t *testing.T) {
	if os.Getenv("RUN_PARSER_ACCURACY") != "1" || os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Skip("set RUN_PARSER_ACCURACY=1 and OPENROUTER_API_KEY to run the live engine accuracy check")
	}
	models := splitModels(os.Getenv("OPENROUTER_MODELS"))
	if len(models) == 0 {
		models = []string{envOrTest("OPENROUTER_MODEL", "google/gemini-flash-1.5-8b")}
	}
	eng, err := NewOpenRouter(OpenRouterConfig{
		BaseURL: os.Getenv("OPENROUTER_BASE_URL"),
		APIKey:  os.Getenv("OPENROUTER_API_KEY"),
		Models:  models,
	}, nil)
	if err != nil {
		t.Fatalf("NewOpenRouter: %v", err)
	}

	cases := []struct {
		name    string
		resume  []string
		nameVal string
		email   string
		phone   string
		inst    string
		company string
		skills  []string
	}{
		{
			name: "english",
			resume: []string{
				"Jane Doe", "jane.doe@example.com", "+62 811 2233 4455",
				"Education: Institut Teknologi Bandung, Bachelor of Science in Computer Science, 2012-2016",
				"Experience: Data Analyst at Traveloka, 2017 - Present",
				"Skills: Python, SQL, Tableau",
			},
			nameVal: "jane doe",
			email:   "jane.doe@example.com",
			phone:   "6281122334455",
			inst:    "institut teknologi bandung",
			company: "traveloka",
			skills:  []string{"python", "sql", "tableau"},
		},
		{
			name: "indonesian",
			resume: []string{
				"Budi Santoso", "budi.santoso@gmail.com", "0812-3456-7890",
				"Pendidikan: Universitas Indonesia, Sarjana Teknik Teknik Informatika, 2014-2018",
				"Pengalaman: Backend Engineer di PT Tokopedia, 2019 - Sekarang",
				"Software Engineer Intern di PT Gojek, 2018-2019",
				"Keahlian: Go, React, Docker, PostgreSQL",
			},
			nameVal: "budi santoso",
			email:   "budi.santoso@gmail.com",
			phone:   "6281234567890",
			inst:    "universitas indonesia",
			company: "tokopedia",
			skills:  []string{"go", "react", "docker", "postgresql"},
		},
	}

	svc := NewService(DefaultExtractor{}, eng, 10<<20)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := svc.Parse(context.Background(), Document{
				Filename:    "resume.docx",
				DocumentB64: docxB64(t, tc.resume),
			})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			checked, matched := 0, 0
			check := func(ok bool) {
				checked++
				if ok {
					matched++
				}
			}
			check(strings.Contains(strings.ToLower(out.Name), tc.nameVal))
			check(strings.EqualFold(strings.TrimSpace(out.Email), tc.email))
			check(digits(out.Phone) == tc.phone)
			check(anyInsContains(out.Education, tc.inst))
			check(anyCompanyContains(out.WorkHistory, tc.company))
			have := map[string]bool{}
			for _, s := range out.Skills {
				have[strings.ToLower(strings.TrimSpace(s))] = true
			}
			for _, s := range tc.skills {
				check(have[s])
			}

			accuracy := float64(matched) / float64(checked)
			if accuracy < 0.9 {
				t.Fatalf("field accuracy %.0f%% (matched %d/%d) below 90%%; extraction: %+v",
					accuracy*100, matched, checked, out)
			}
		})
	}
}

func envOrTest(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func splitModels(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func anyInsContains(entries []EducationEntry, want string) bool {
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Institution), want) {
			return true
		}
	}
	return false
}

func anyCompanyContains(entries []WorkEntry, want string) bool {
	for _, w := range entries {
		if strings.Contains(strings.ToLower(w.Company), want) {
			return true
		}
	}
	return false
}
