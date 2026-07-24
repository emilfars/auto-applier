// Package seed generates deterministic synthetic Jabodetabek job listings used
// to populate the feed before signup opens (plan: seed >= 5,000 listings to
// avoid empty-feed churn) and to drive the filter performance gate (AC-FEED-2b).
//
// Listings are Jabodetabek-first and carry employer-stated pay only — the
// generator never fabricates an estimated salary (locked decision). Data is
// produced by combinatorial indexing so every listing has a unique
// (company, title, city) identity and therefore a distinct dedup key; no two
// generated jobs collapse into one.
package seed

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/auto-applier/backend/internal/ingest"
)

// Jabodetabek-first cities. cityOf() in the normalizer reads the text before the
// first comma, so each Location is "<City>, <Province>".
var cities = []struct{ city, province string }{
	{"Jakarta Pusat", "DKI Jakarta"},
	{"Jakarta Selatan", "DKI Jakarta"},
	{"Jakarta Barat", "DKI Jakarta"},
	{"Jakarta Timur", "DKI Jakarta"},
	{"Jakarta Utara", "DKI Jakarta"},
	{"Bogor", "Jawa Barat"},
	{"Depok", "Jawa Barat"},
	{"Tangerang", "Banten"},
	{"Tangerang Selatan", "Banten"},
	{"Bekasi", "Jawa Barat"},
}

var titles = []string{
	"Software Engineer", "Backend Engineer", "Frontend Engineer",
	"Full Stack Developer", "Mobile Engineer", "Data Analyst",
	"Data Engineer", "Data Scientist", "Product Manager",
	"Product Designer", "UX Researcher", "QA Engineer",
	"DevOps Engineer", "Site Reliability Engineer", "Security Engineer",
	"Business Analyst", "Digital Marketing Specialist", "Content Writer",
	"Social Media Officer", "Accountant", "Finance Staff", "HR Generalist",
	"Recruiter", "Customer Success Officer", "Customer Service Representative",
	"Sales Executive", "Account Manager", "Operations Staff",
	"Project Manager", "Graphic Designer", "Video Editor",
	"Machine Learning Engineer", "Android Developer", "iOS Developer",
	"Network Engineer", "Database Administrator", "Technical Writer",
	"Growth Marketer", "Legal Officer", "Procurement Staff",
}

var companies = []string{
	"Nusantara Digital", "Jaya Teknologi", "Sinar Data", "Cakra Solusi",
	"Garuda Analytics", "Bahtera Fintech", "Meridian Labs", "Andalan Group",
	"Mitra Cerdas", "Kirana Media", "Prima Logistik", "Sentosa Retail",
	"Wana Energi", "Bumi Kreasi", "Cendana Bank", "Delta Payments",
	"Elang Commerce", "Fajar Studio", "Gemilang Health", "Harmoni Edu",
	"Insan Mandiri", "Juwita Beauty", "Kartika Foods", "Lentera Cloud",
	"Mustika Travel", "Nirwana Property", "Oasis Agritech", "Pelita Insurance",
	"Rajawali Auto", "Selaras HR", "Teratai Games", "Untung Marketplace",
	"Vega Robotics", "Wijaya Manufaktur", "Xenia Mobility", "Yudha Security",
	"Zamrud Ventures", "Angkasa Aero", "Buana Maritim", "Cahaya Solar",
}

var employmentTypes = []string{"Full Time", "Kontrak", "Paruh Waktu", "Magang", "Freelance"}

var seniorities = []struct {
	label    string
	minYoE   int
	titleTag string
}{
	{"Fresh Graduate", 0, ""},
	{"Junior", 1, "Junior"},
	{"Mid", 3, ""},
	{"Senior", 5, "Senior"},
	{"Lead", 7, "Lead"},
	{"Manager", 8, "Manager"},
}

// Tier 1/2 source ids only (Tier 3 login-walled sources are excluded).
var sources = []string{"kalibrr", "glints", "jobstreet", "kemnaker", "company-career"}

var skillPool = []string{
	"Go", "Python", "JavaScript", "TypeScript", "React", "SQL", "PostgreSQL",
	"Docker", "Kubernetes", "AWS", "GCP", "Java", "Kotlin", "Swift", "Figma",
	"Excel", "Tableau", "Communication", "English", "Bahasa Indonesia",
}

// Generator produces synthetic listings deterministically. The same seed always
// yields the same listings, so tests and perf gates are reproducible.
type Generator struct {
	seed int64
	// baseTime anchors PostedAt so generated dates are stable across runs.
	baseTime time.Time
}

// NewGenerator returns a Generator with the given seed. baseTime anchors the
// (deterministic) posting dates; pass a fixed time in tests.
func NewGenerator(seed int64, baseTime time.Time) *Generator {
	return &Generator{seed: seed, baseTime: baseTime.UTC()}
}

// RawJob returns the i-th raw listing. Identity fields (company/title/city) are
// derived by combinatorial indexing so every i maps to a unique dedup key;
// remaining fields vary via a per-index PRNG for realistic diversity.
func (g *Generator) RawJob(i int) ingest.RawJob {
	nCity, nTitle, nCompany := len(cities), len(titles), len(companies)

	loc := cities[i%nCity]
	title := titles[(i/nCity)%nTitle]
	companyBase := companies[(i/(nCity*nTitle))%nCompany]
	group := i / (nCity * nTitle * nCompany)
	company := companyBase
	if group > 0 {
		company = fmt.Sprintf("%s %d", companyBase, group+1)
	}

	r := rand.New(rand.NewSource(g.seed + int64(i)*2654435761))

	sen := seniorities[r.Intn(len(seniorities))]
	displayTitle := title
	if sen.titleTag != "" {
		displayTitle = sen.titleTag + " " + title
	}

	remote := r.Intn(100) < 15
	location := fmt.Sprintf("%s, %s", loc.city, loc.province)

	// Requirements: a handful of skills plus (usually) a stated YoE phrase so the
	// normalizer can populate YearsExperience for the YoE filter.
	reqs := pickSkills(r, 2+r.Intn(4))
	yoe := sen.minYoE
	if r.Intn(100) < 80 { // ~20% of listings state no explicit YoE
		reqs = append(reqs, fmt.Sprintf("Minimum %d years experience", yoe))
	}

	posted := g.baseTime.Add(-time.Duration(r.Intn(30*24)) * time.Hour)

	return ingest.RawJob{
		Source:         sources[r.Intn(len(sources))],
		SourceURL:      fmt.Sprintf("https://example.test/%s/%d", companyBase, i),
		Title:          displayTitle,
		Company:        company,
		Location:       location,
		Remote:         remote,
		SalaryText:     salaryText(r),
		Seniority:      sen.label,
		EmploymentType: employmentTypes[r.Intn(len(employmentTypes))],
		Requirements:   reqs,
		PostedAt:       &posted,
	}
}

// Jobs returns n normalized listings. It is a convenience for tests and perf
// benchmarks that need the data without a store.
func (g *Generator) Jobs(n int) []ingest.Job {
	jobs := make([]ingest.Job, 0, n)
	for i := 0; i < n; i++ {
		j, err := ingest.Normalize(g.RawJob(i))
		if err != nil {
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs
}

func pickSkills(r *rand.Rand, k int) []string {
	if k > len(skillPool) {
		k = len(skillPool)
	}
	perm := r.Perm(len(skillPool))[:k]
	out := make([]string, 0, k)
	for _, idx := range perm {
		out = append(out, skillPool[idx])
	}
	return out
}

// salaryText returns an employer-stated IDR range, or a "negotiable" label for a
// minority of listings (which the normalizer maps to no stated pay — never an
// estimate).
func salaryText(r *rand.Rand) string {
	if r.Intn(100) < 12 {
		return "Gaji Negosiasi"
	}
	// Bands in millions of IDR.
	lo := 4 + r.Intn(20)      // 4jt..23jt
	hi := lo + 2 + r.Intn(15) // lo+2 .. lo+16
	return fmt.Sprintf("Rp %d.000.000 - Rp %d.000.000", lo, hi)
}
