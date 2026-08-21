package ingest

import (
	"net/http"
	"testing"
)

// AC-SCR-10: Jooble's lifetime-quota credential is not included in recurring
// registry construction unless the caller explicitly requests backfill.
func TestBuildRegistryKeepsJoobleBackfillOnly(t *testing.T) {
	withoutBackfill := BuildRegistry(SourceConfig{
		JoobleAPIKey: "jooble-secret",
		HTTPClient:   &http.Client{},
	})
	if len(withoutBackfill.Sources()) != 0 {
		t.Fatalf("recurring registry sources = %d, want 0", len(withoutBackfill.Sources()))
	}

	withBackfill := BuildRegistry(SourceConfig{
		JoobleAPIKey:   "jooble-secret",
		JoobleBackfill: true,
		HTTPClient:     &http.Client{},
	})
	if len(withBackfill.Sources()) != 1 || withBackfill.Sources()[0].ID() != "jooble" {
		t.Fatalf("backfill registry = %v, want Jooble only", sourceIDs(withBackfill.Sources()))
	}
}

func TestBuildRegistryIncludesM3SupplementsWhenEnabled(t *testing.T) {
	registry := BuildRegistry(SourceConfig{
		CareerjetAffid: "careerjet-affid",
		CareerjetLimit: 10,
		ATSSlugs: map[ATSPlatform][]string{
			ATSGreenhouse: {"xendit"},
			ATSLever:      {},
			ATSWorkable:   {},
			ATSAshby:      {},
		},
		RemoteEnabled: true,
		RemoteLimit:   10,
		HTTPClient:    &http.Client{},
	})
	ids := sourceIDs(registry.Sources())
	for _, want := range []string{"careerjet", "ats-greenhouse-xendit", "remotive", "jobicy", "remoteok"} {
		if !contains(ids, want) {
			t.Fatalf("registry ids=%v, missing %q", ids, want)
		}
	}
	if contains(ids, "jooble") {
		t.Fatal("Jooble leaked into recurring supplement registry")
	}
}

func sourceIDs(sources []Source) []string {
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.ID())
	}
	return ids
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
