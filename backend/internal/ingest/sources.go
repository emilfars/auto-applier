package ingest

import (
	"log"
	"net/http"
	"time"
)

// SourceConfig configures which real sources are enabled. Values are supplied
// by the caller (from env), keeping this package free of os/env coupling.
type SourceConfig struct {
	// KalibrrLimit caps how many Kalibrr listings to pull per run (<=0 skips
	// Kalibrr). Kalibrr needs no key.
	KalibrrLimit int
	// JoobleAPIKey enables the Tier 1 Jooble source when non-empty. Without it
	// the source is not registered (graceful degradation).
	JoobleAPIKey string
	// JoobleLimit caps how many Jooble listings to pull per run. A non-positive
	// value falls back to KalibrrLimit for compatibility with existing callers.
	JoobleLimit int
	// HTTPClient is shared by all sources; nil uses a default with a timeout.
	HTTPClient *http.Client
}

// BuildRegistry assembles the enabled real sources per the sourcing-reality
// decisions: Kalibrr (Tier 2, keyless) is always registered when a limit is
// set; Jooble (Tier 1) is registered only when an API key is provided. Logins
// walled boards are never wired in. It returns an empty registry (no error) if
// nothing is enabled, so callers degrade gracefully.
func BuildRegistry(cfg SourceConfig) *Registry {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	reg := NewRegistry()

	if cfg.KalibrrLimit > 0 {
		if err := reg.Register(NewKalibrrSource(cfg.KalibrrLimit, client)); err != nil {
			log.Printf("ingest: register kalibrr: %v", err)
		}
	}
	if cfg.JoobleAPIKey != "" {
		joobleLimit := cfg.JoobleLimit
		if joobleLimit <= 0 {
			joobleLimit = cfg.KalibrrLimit
		}
		if err := reg.Register(NewJoobleSourceWithLimit(cfg.JoobleAPIKey, joobleLimit, client)); err != nil {
			log.Printf("ingest: register jooble: %v", err)
		}
	}
	return reg
}
