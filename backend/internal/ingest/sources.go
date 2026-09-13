package ingest

import (
	"embed"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// The slug catalog is data rather than code so adding a public company board
// does not require changing an adapter.
//
//go:embed atscompanies/*.json
var atsCompanyFiles embed.FS

// ATSCompany is one curated public-board company entry.
type ATSCompany struct {
	Slug    string `json:"slug"`
	Company string `json:"company"`
}

// SourceConfig configures which real sources are enabled. Values are supplied
// by the caller (from env), keeping this package free of os/env coupling.
type SourceConfig struct {
	// KalibrrLimit caps how many Kalibrr listings to pull per run (<=0 skips
	// Kalibrr). Kalibrr needs no key.
	KalibrrLimit int
	// JoobleAPIKey supplies the Tier 1 backfill credential when non-empty. The
	// source is still not registered unless JoobleBackfill is explicitly true.
	JoobleAPIKey string
	// JoobleLimit caps how many Jooble listings to pull per run. A non-positive
	// value falls back to KalibrrLimit for compatibility with existing callers.
	JoobleLimit int
	// JoobleBackfill explicitly enables Jooble. Jooble's lifetime quota makes it
	// unsuitable for the recurring sweep, so the API queue leaves this false and
	// the real seed command sets it true for initial backfill.
	JoobleBackfill bool
	// CareerjetAffid enables the keyed Careerjet Tier 1 source when non-empty.
	CareerjetAffid string
	// CareerjetLimit caps Careerjet listings per run. A non-positive value uses
	// the source default.
	CareerjetLimit int
	// ATSEnabled enables one public source per company slug in the curated
	// catalog. ATSSlugs overrides the catalog for the listed platform, which is
	// useful for fixture tests and controlled deployments. WorkdayCompanies and
	// SmartRecruitersCompanies override the workday/smartrecruiters catalogs the
	// same way.
	ATSEnabled bool
	ATSSlugs   map[ATSPlatform][]string
	// WorkdayCompanies overrides the embedded Workday catalog when non-nil.
	WorkdayCompanies []WorkdayCompany
	// SmartRecruitersCompanies overrides the SmartRecruiters catalog when non-nil.
	SmartRecruitersCompanies []SmartRecruitersCompany
	// RemoteEnabled enables the keyless Remotive, Jobicy, and RemoteOK sources.
	RemoteEnabled bool
	RemoteLimit   int
	// HTTPClient is shared by all sources; nil uses a pooled default.
	HTTPClient *http.Client
}

// NewPooledHTTPClient returns a shared client tuned for harvesting many small
// JSON boards: bounded per-host idle connections (keep-alive reuse instead of a
// TCP+TLS handshake per request), a global idle cap, and an overall timeout.
func NewPooledHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second}
}

// BuildRegistry assembles enabled real sources. Keyed sources degrade when
// their credentials are absent, and Jooble is deliberately backfill-only.
// Login-walled boards are never wired in. It returns an empty registry (no
// error) if nothing is enabled, so callers degrade gracefully.
func BuildRegistry(cfg SourceConfig) *Registry {
	client := cfg.HTTPClient
	if client == nil {
		client = NewPooledHTTPClient()
	}
	reg := NewRegistry()

	if cfg.KalibrrLimit > 0 {
		if err := reg.Register(NewKalibrrSource(cfg.KalibrrLimit, client)); err != nil {
			log.Printf("ingest: register kalibrr: %v", err)
		}
	}
	if cfg.JoobleBackfill && strings.TrimSpace(cfg.JoobleAPIKey) != "" {
		joobleLimit := cfg.JoobleLimit
		if joobleLimit <= 0 {
			joobleLimit = cfg.KalibrrLimit
		}
		if err := reg.Register(NewJoobleSourceWithLimit(cfg.JoobleAPIKey, joobleLimit, client)); err != nil {
			log.Printf("ingest: register jooble: %v", err)
		}
	}
	if strings.TrimSpace(cfg.CareerjetAffid) != "" {
		if err := reg.Register(NewCareerjetSourceWithLimit(cfg.CareerjetAffid, cfg.CareerjetLimit, client)); err != nil {
			log.Printf("ingest: register careerjet: %v", err)
		}
	}
	if cfg.ATSEnabled || len(cfg.ATSSlugs) > 0 || cfg.WorkdayCompanies != nil || cfg.SmartRecruitersCompanies != nil {
		for _, platform := range []ATSPlatform{ATSGreenhouse, ATSLever, ATSWorkable, ATSAshby} {
			entries := configuredATSCompanies(platform, cfg.ATSSlugs)
			for _, entry := range entries {
				source := NewATSSourceWithURL(platform, entry.Slug, entry.Company, atsEndpoint(platform, entry.Slug), client)
				if err := reg.Register(source); err != nil {
					log.Printf("ingest: register %s/%s: %v", platform, entry.Slug, err)
				}
			}
		}
		for _, entry := range configuredWorkdayCompanies(cfg.WorkdayCompanies) {
			source := NewWorkdaySource(entry, client)
			if err := reg.Register(source); err != nil {
				log.Printf("ingest: register workday/%s: %v", entry.Site, err)
			}
		}
		for _, entry := range configuredSmartRecruitersCompanies(cfg.SmartRecruitersCompanies) {
			source := NewSmartRecruitersSource(entry, client)
			if err := reg.Register(source); err != nil {
				log.Printf("ingest: register smartrecruiters/%s: %v", entry.Slug, err)
			}
		}
	}
	if cfg.RemoteEnabled {
		for _, source := range []Source{
			NewRemotiveSource(cfg.RemoteLimit, client),
			NewJobicySource(cfg.RemoteLimit, client),
			NewRemoteOKSource(cfg.RemoteLimit, client),
		} {
			if err := reg.Register(source); err != nil {
				log.Printf("ingest: register %s: %v", source.ID(), err)
			}
		}
	}
	return reg
}

func configuredATSCompanies(platform ATSPlatform, overrides map[ATSPlatform][]string) []ATSCompany {
	if slugs, ok := overrides[platform]; ok {
		entries := make([]ATSCompany, 0, len(slugs))
		for _, slug := range slugs {
			if strings.TrimSpace(slug) != "" {
				entries = append(entries, ATSCompany{Slug: strings.TrimSpace(slug)})
			}
		}
		return entries
	}

	var entries []ATSCompany
	path := "atscompanies/" + string(platform) + ".json"
	data, err := atsCompanyFiles.ReadFile(path)
	if err != nil {
		log.Printf("ingest: read ATS catalog %s: %v", path, err)
		return nil
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("ingest: decode ATS catalog %s: %v", path, err)
		return nil
	}
	return entries
}

// configuredWorkdayCompanies returns the override list when provided, else the
// embedded catalog. Invalid entries (missing host/tenant/site) are dropped so a
// typo cannot register a source that can never fetch.
func configuredWorkdayCompanies(overrides []WorkdayCompany) []WorkdayCompany {
	entries := overrides
	if entries == nil {
		if err := readATSCatalog("workday", &entries); err != nil {
			log.Printf("ingest: read ATS catalog workday.json: %v", err)
			return nil
		}
	}
	out := make([]WorkdayCompany, 0, len(entries))
	for _, entry := range entries {
		entry.Host = strings.TrimSpace(entry.Host)
		entry.Tenant = strings.TrimSpace(entry.Tenant)
		entry.Site = strings.TrimSpace(entry.Site)
		if entry.Host == "" || entry.Tenant == "" || entry.Site == "" {
			log.Printf("ingest: skipping invalid Workday catalog entry %+v", entry)
			continue
		}
		out = append(out, entry)
	}
	return out
}

// configuredSmartRecruitersCompanies returns the override list when provided,
// else the embedded catalog.
func configuredSmartRecruitersCompanies(overrides []SmartRecruitersCompany) []SmartRecruitersCompany {
	entries := overrides
	if entries == nil {
		if err := readATSCatalog("smartrecruiters", &entries); err != nil {
			log.Printf("ingest: read ATS catalog smartrecruiters.json: %v", err)
			return nil
		}
	}
	out := make([]SmartRecruitersCompany, 0, len(entries))
	for _, entry := range entries {
		entry.Slug = strings.TrimSpace(entry.Slug)
		if entry.Slug == "" {
			log.Printf("ingest: skipping invalid SmartRecruiters catalog entry %+v", entry)
			continue
		}
		out = append(out, entry)
	}
	return out
}

func readATSCatalog(name string, v any) error {
	path := "atscompanies/" + name + ".json"
	data, err := atsCompanyFiles.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
