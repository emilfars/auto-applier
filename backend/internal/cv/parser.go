package cv

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

// ParsedCV is the normalized structured result of parsing a CV. It carries the
// contact, education, work-history and skills fields required by AC-CV-2. The
// data is never treated as authoritative: the user always reviews and edits it
// (AC-CV-3) and must confirm before any fill flow is armed (AC-CV-5).
type ParsedCV struct {
	FullName    string           `json:"full_name"`
	Email       string           `json:"email"`
	Phone       string           `json:"phone"`
	Education   []EducationEntry `json:"education"`
	WorkHistory []WorkEntry      `json:"work_history"`
	Skills      []string         `json:"skills"`
}

// EducationEntry is one schooling record.
type EducationEntry struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Field       string `json:"field"`
	StartYear   string `json:"start_year"`
	EndYear     string `json:"end_year"`
}

// WorkEntry is one employment record.
type WorkEntry struct {
	Company   string `json:"company"`
	Title     string `json:"title"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// Parser turns raw CV bytes into a ParsedCV. Implementations may call a hosted
// resume-parsing API (production) or return canned data (tests). The parser is
// deliberately behind an interface so the rest of the system never depends on a
// specific vendor.
type Parser interface {
	Parse(ctx context.Context, data []byte, filename, contentType string) (ParsedCV, error)
}

var (
	// keep digits and a single leading +; strip spaces, dashes, dots, parens.
	phoneStrip = regexp.MustCompile(`[^\d+]`)
	multiSpace = regexp.MustCompile(`\s+`)
)

// normalize cleans a ParsedCV in place-independent fashion so equivalent inputs
// (differing only in whitespace, casing, phone punctuation, or duplicate skills)
// map to one canonical form. This is the deterministic, vendor-agnostic core
// exercised by the accuracy fixtures.
func normalize(p ParsedCV) ParsedCV {
	out := ParsedCV{
		FullName: collapse(p.FullName),
		Email:    strings.ToLower(collapse(p.Email)),
		Phone:    normalizePhone(p.Phone),
	}
	for _, e := range p.Education {
		e = EducationEntry{
			Institution: collapse(e.Institution),
			Degree:      collapse(e.Degree),
			Field:       collapse(e.Field),
			StartYear:   collapse(e.StartYear),
			EndYear:     normalizeEndYear(e.EndYear),
		}
		if e != (EducationEntry{}) {
			out.Education = append(out.Education, e)
		}
	}
	for _, w := range p.WorkHistory {
		w = WorkEntry{
			Company:   collapse(w.Company),
			Title:     collapse(w.Title),
			StartDate: collapse(w.StartDate),
			EndDate:   normalizeEndYear(w.EndDate),
		}
		if w != (WorkEntry{}) {
			out.WorkHistory = append(out.WorkHistory, w)
		}
	}
	out.Skills = normalizeSkills(p.Skills)
	return out
}

// collapse trims and collapses internal whitespace.
func collapse(s string) string {
	return multiSpace.ReplaceAllString(strings.TrimSpace(s), " ")
}

// normalizePhone reduces a phone number to digits with an optional leading +,
// and rewrites a leading Indonesian 0 to the +62 country code so numbers from
// local-format CVs compare equal to internationally-formatted ones.
func normalizePhone(s string) string {
	s = phoneStrip.ReplaceAllString(s, "")
	if s == "" {
		return ""
	}
	// A stray + not at the front is meaningless; keep only a leading one.
	plus := strings.HasPrefix(s, "+")
	digits := strings.ReplaceAll(s, "+", "")
	switch {
	case plus:
		return "+" + digits
	case strings.HasPrefix(digits, "62"):
		return "+" + digits
	case strings.HasPrefix(digits, "0"):
		return "+62" + strings.TrimPrefix(digits, "0")
	default:
		return digits
	}
}

// normalizeEndYear canonicalises open-ended ranges ("present", "sekarang",
// "current", "now", "-") to the empty string so an ongoing role/study compares
// equal regardless of wording/language.
func normalizeEndYear(s string) string {
	s = collapse(s)
	switch strings.ToLower(s) {
	case "present", "sekarang", "current", "now", "-", "":
		return ""
	default:
		return s
	}
}

// normalizeSkills trims, drops blanks, de-duplicates case-insensitively (keeping
// the first-seen casing), and sorts for a stable order.
func normalizeSkills(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = collapse(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}
