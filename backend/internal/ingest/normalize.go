package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ErrIncomplete is returned by Normalize when a required field is missing.
var ErrIncomplete = errors.New("ingest: raw job missing required field")

var (
	wsRe        = regexp.MustCompile(`\s+`)
	nonAlnumRe  = regexp.MustCompile(`[^a-z0-9]+`)
	floatTokRe  = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
	intTokRe    = regexp.MustCompile(`\d[\d.,]*`)
	remoteWords = []string{"remote", "wfh", "work from home", "kerja dari rumah", "wfa", "anywhere"}
)

// Normalize converts a RawJob into a normalized Job (AC-SCR-2). It validates the
// required fields, parses stated salary (IDR), and derives a cross-source dedup
// key (AC-SCR-3). It never invents estimated salary — stated-only at MVP.
func Normalize(raw RawJob) (Job, error) {
	source := collapse(raw.Source)
	url := strings.TrimSpace(raw.SourceURL)
	title := collapse(raw.Title)
	company := collapse(raw.Company)
	if source == "" || url == "" || title == "" || company == "" {
		return Job{}, ErrIncomplete
	}

	location := collapse(raw.Location)
	remote := raw.Remote || detectRemote(location) || detectRemote(title)

	min, max := ParseSalaryIDR(raw.SalaryText)

	reqs := make([]string, 0, len(raw.Requirements))
	for _, r := range raw.Requirements {
		if c := collapse(r); c != "" {
			reqs = append(reqs, c)
		}
	}

	j := Job{
		Source:          source,
		SourceURL:       url,
		Title:           title,
		Company:         company,
		Location:        location,
		Remote:          remote,
		SalaryStatedMin: min,
		SalaryStatedMax: max,
		SalaryCurrency:  "IDR",
		Seniority:       normalizeSeniority(raw.Seniority),
		EmploymentType:  normalizeEmploymentType(raw.EmploymentType),
		Requirements:    reqs,
		PostedAt:        raw.PostedAt,
	}
	j.DedupKey = dedupKey(company, title, location)
	return j, nil
}

func collapse(s string) string {
	return wsRe.ReplaceAllString(strings.TrimSpace(s), " ")
}

func detectRemote(s string) bool {
	l := strings.ToLower(s)
	for _, w := range remoteWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// dedupKey collapses a listing to a canonical identity so the same job posted on
// multiple boards maps to one key (SCR-3). It is city/company/title based and
// intentionally ignores the source.
func dedupKey(company, title, location string) string {
	canon := func(s string) string {
		return strings.Trim(nonAlnumRe.ReplaceAllString(strings.ToLower(s), " "), " ")
	}
	seed := canon(company) + "|" + canon(title) + "|" + canon(cityOf(location))
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16])
}

// cityOf reduces a location to its primary city token (before the first comma).
func cityOf(location string) string {
	if i := strings.IndexByte(location, ','); i >= 0 {
		return strings.TrimSpace(location[:i])
	}
	return location
}

var employmentSynonyms = map[string]string{
	"full time": "full_time", "fulltime": "full_time", "full-time": "full_time",
	"penuh waktu": "full_time", "tetap": "full_time", "permanent": "full_time",
	"part time": "part_time", "part-time": "part_time", "paruh waktu": "part_time",
	"contract": "contract", "kontrak": "contract",
	"internship": "internship", "intern": "internship", "magang": "internship",
	"freelance": "freelance", "lepas": "freelance",
	"temporary": "temporary", "sementara": "temporary",
}

func normalizeEmploymentType(s string) string {
	key := strings.ToLower(collapse(s))
	if key == "" {
		return ""
	}
	if v, ok := employmentSynonyms[key]; ok {
		return v
	}
	// Fall back to a loose contains match so decorated labels still map.
	for syn, v := range employmentSynonyms {
		if strings.Contains(key, syn) {
			return v
		}
	}
	return ""
}

var senioritySynonyms = []struct {
	needle string
	value  string
}{
	{"intern", "intern"}, {"magang", "intern"},
	{"fresh grad", "entry"}, {"entry", "entry"}, {"pemula", "entry"},
	{"junior", "junior"},
	{"mid", "mid"}, {"intermediate", "mid"}, {"menengah", "mid"},
	{"principal", "lead"}, {"lead", "lead"},
	{"manager", "manager"}, {"head", "manager"}, {"kepala", "manager"},
	{"senior", "senior"},
}

func normalizeSeniority(s string) string {
	key := strings.ToLower(collapse(s))
	if key == "" {
		return ""
	}
	for _, m := range senioritySynonyms {
		if strings.Contains(key, m.needle) {
			return m.value
		}
	}
	return key
}

// ParseSalaryIDR extracts a stated salary range from free-text. It handles
// Indonesian formats: dotted thousands ("Rp 8.000.000 - 12.000.000") and the
// "juta"/"jt" million shorthand ("8 - 12 juta", "Rp 5jt"). A single value yields
// an equal min and max. Non-numeric or negotiable text yields (nil, nil) — the
// system never fabricates a figure.
func ParseSalaryIDR(text string) (*int64, *int64) {
	s := strings.ToLower(text)
	if s == "" {
		return nil, nil
	}
	for _, skip := range []string{"nego", "competitive", "disclos", "undisclosed", "kompetitif"} {
		if strings.Contains(s, skip) {
			return nil, nil
		}
	}

	millions := strings.Contains(s, "juta") || regexp.MustCompile(`\d\s*jt\b`).MatchString(s)

	var vals []int64
	if millions {
		for _, tok := range floatTokRe.FindAllString(s, -1) {
			f, err := strconv.ParseFloat(strings.Replace(tok, ",", ".", 1), 64)
			if err != nil {
				continue
			}
			vals = append(vals, int64(f*1_000_000))
		}
	} else {
		for _, tok := range intTokRe.FindAllString(s, -1) {
			clean := strings.NewReplacer(".", "", ",", "").Replace(tok)
			n, err := strconv.ParseInt(clean, 10, 64)
			if err != nil {
				continue
			}
			vals = append(vals, n)
		}
	}

	switch len(vals) {
	case 0:
		return nil, nil
	case 1:
		v := vals[0]
		return &v, &v
	default:
		lo, hi := vals[0], vals[len(vals)-1]
		if lo > hi {
			lo, hi = hi, lo
		}
		return &lo, &hi
	}
}
