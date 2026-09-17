// Command discover-boards grows the ATS board catalogs (M7) from the Wayback
// Machine CDX index, validating every candidate against its public API before
// it is added. It defaults to a dry run; pass -write to persist changes.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/discover"
)

func main() {
	platformsFlag := flag.String("platforms", "", "comma-separated platforms (default: all)")
	limit := flag.Int("limit", 200, "max CDX results per URL pattern")
	outDir := flag.String("out", "internal/ingest/atscompanies", "catalog output directory")
	write := flag.Bool("write", false, "persist validated candidates to the catalogs")
	allowGlobal := flag.Bool("allow-global", false, "keep boards with no Indonesian-location signal (default: drop them)")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall run timeout")
	flag.Parse()

	platforms, err := parsePlatforms(*platformsFlag)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report, err := discover.Run(ctx, discover.Config{
		Platforms:   platforms,
		Limit:       *limit,
		OutDir:      *outDir,
		Write:       *write,
		AllowGlobal: *allowGlobal,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range discover.Platforms() {
		pr, ok := report.Platforms[p]
		if !ok {
			continue
		}
		status := "ok"
		if pr.Err != nil {
			status = "error: " + pr.Err.Error()
		}
		log.Printf("discover %-16s discovered=%d validated=%d added=%d total=%d %s",
			p, pr.Discovered, pr.Validated, pr.Added, pr.Total, status)
	}
	if !*write {
		log.Printf("dry run: pass -write to update catalogs in %s", *outDir)
	}
}

func parsePlatforms(raw string) ([]discover.Platform, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	valid := map[discover.Platform]bool{}
	for _, p := range discover.Platforms() {
		valid[p] = true
	}
	var out []discover.Platform
	for _, part := range strings.Split(raw, ",") {
		p := discover.Platform(strings.TrimSpace(part))
		if !valid[p] {
			return nil, fmt.Errorf("unknown platform %q", p)
		}
		out = append(out, p)
	}
	return out, nil
}
