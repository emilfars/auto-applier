package cv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestParseAccuracy validates AC-CV-2: mapping a hosted-API response into a
// ParsedCV must populate contact/education/work/skills with >=90% field
// accuracy against labeled fixtures, for both Indonesian and English CVs.
func TestParseAccuracy(t *testing.T) {
	const threshold = 0.90
	for _, lang := range []string{"id", "en"} {
		t.Run(lang, func(t *testing.T) {
			var raw hostedResponse
			readJSON(t, lang+"_response.json", &raw)
			var want ParsedCV
			readJSON(t, lang+"_expected.json", &want)

			got := raw.toParsedCV()
			acc := fieldAccuracy(want, got)
			if acc < threshold {
				t.Fatalf("field accuracy %.2f < %.2f\n got:  %+v\n want: %+v", acc, threshold, got, want)
			}
			t.Logf("%s field accuracy: %.2f", lang, acc)
		})
	}
}

// TestParseAccuracyExact is stricter than the AC gate: because normalization is
// deterministic, the mapped output should match the labeled fixture exactly.
// This guards against silent regressions in the mapping/normalization.
func TestParseAccuracyExact(t *testing.T) {
	for _, lang := range []string{"id", "en"} {
		t.Run(lang, func(t *testing.T) {
			var raw hostedResponse
			readJSON(t, lang+"_response.json", &raw)
			var want ParsedCV
			readJSON(t, lang+"_expected.json", &want)
			got := raw.toParsedCV()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("mapped CV != fixture\n got:  %+v\n want: %+v", got, want)
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"0812-3456-7890":    "+6281234567890",
		"+62 811 2233 4455": "+6281122334455",
		"(021) 555 1234":    "+62215551234",
		"6281234567890":     "+6281234567890",
		"":                  "",
		"12345":             "12345",
	}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeSkills(t *testing.T) {
	got := normalizeSkills([]string{"Go", "go", " React ", "", "Docker", "docker", "PostgreSQL"})
	want := []string{"Docker", "Go", "PostgreSQL", "React"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSkills = %v, want %v", got, want)
	}
}

func TestNormalizeEndYear(t *testing.T) {
	for _, in := range []string{"Present", "sekarang", "current", "now", "-", ""} {
		if got := normalizeEndYear(in); got != "" {
			t.Errorf("normalizeEndYear(%q) = %q, want empty", in, got)
		}
	}
	if got := normalizeEndYear(" 2019 "); got != "2019" {
		t.Errorf("normalizeEndYear(2019) = %q", got)
	}
}

// fieldAccuracy computes the fraction of leaf fields in want that got reproduces
// correctly. Missing/extra list items and mismatched skills count against the
// score, matching the intuitive notion of parse accuracy.
func fieldAccuracy(want, got ParsedCV) float64 {
	total, match := 0, 0
	eq := func(a, b string) {
		total++
		if a == b {
			match++
		}
	}
	eq(want.FullName, got.FullName)
	eq(want.Email, got.Email)
	eq(want.Phone, got.Phone)

	n := max(len(want.Education), len(got.Education))
	for i := 0; i < n; i++ {
		var a, b EducationEntry
		if i < len(want.Education) {
			a = want.Education[i]
		}
		if i < len(got.Education) {
			b = got.Education[i]
		}
		eq(a.Institution, b.Institution)
		eq(a.Degree, b.Degree)
		eq(a.Field, b.Field)
		eq(a.StartYear, b.StartYear)
		eq(a.EndYear, b.EndYear)
	}

	m := max(len(want.WorkHistory), len(got.WorkHistory))
	for i := 0; i < m; i++ {
		var a, b WorkEntry
		if i < len(want.WorkHistory) {
			a = want.WorkHistory[i]
		}
		if i < len(got.WorkHistory) {
			b = got.WorkHistory[i]
		}
		eq(a.Company, b.Company)
		eq(a.Title, b.Title)
		eq(a.StartDate, b.StartDate)
		eq(a.EndDate, b.EndDate)
	}

	gotSet := map[string]bool{}
	for _, s := range got.Skills {
		gotSet[strings.ToLower(s)] = true
	}
	union := map[string]bool{}
	for _, s := range want.Skills {
		union[strings.ToLower(s)] = true
	}
	for _, s := range got.Skills {
		union[strings.ToLower(s)] = true
	}
	for s := range union {
		total++
		wantHas := false
		for _, w := range want.Skills {
			if strings.ToLower(w) == s {
				wantHas = true
				break
			}
		}
		if wantHas && gotSet[s] {
			match++
		}
	}

	if total == 0 {
		return 1
	}
	return float64(match) / float64(total)
}

func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
}
