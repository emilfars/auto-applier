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

// AC-SCR-11/12: Workday and SmartRecruiters boards wired via overrides are
// registered as Tier 2 sources next to the existing ATS harvesters.
func TestBuildRegistryIncludesWorkdayAndSmartRecruiters(t *testing.T) {
	registry := BuildRegistry(SourceConfig{
		WorkdayCompanies: []WorkdayCompany{
			{Host: "acme.wd3.myworkdayjobs.com", Tenant: "acme", Site: "Acme_Careers", Company: "Acme"},
		},
		SmartRecruitersCompanies: []SmartRecruitersCompany{
			{Slug: "Cermati", Company: "Cermati.com"},
		},
		HTTPClient: &http.Client{},
	})
	ids := sourceIDs(registry.Sources())
	for _, want := range []string{"workday-acme-acme-careers", "smartrecruiters-cermati"} {
		if !contains(ids, want) {
			t.Fatalf("registry ids=%v, missing %q", ids, want)
		}
	}
}

func TestWorkdayAndSmartRecruitersCatalogsLoad(t *testing.T) {
	workday := configuredWorkdayCompanies(nil)
	if len(workday) == 0 {
		t.Fatal("workday catalog is empty")
	}
	for _, entry := range workday {
		if entry.Host == "" || entry.Tenant == "" || entry.Site == "" || entry.Company == "" {
			t.Fatalf("invalid Workday catalog entry: %+v", entry)
		}
	}
	smart := configuredSmartRecruitersCompanies(nil)
	if len(smart) == 0 {
		t.Fatal("smartrecruiters catalog is empty")
	}
	for _, entry := range smart {
		if entry.Slug == "" || entry.Company == "" {
			t.Fatalf("invalid SmartRecruiters catalog entry: %+v", entry)
		}
	}
}

// AC-SCR-13: the shared harvester client keeps per-host idle connections so a
// board sweep reuses TCP+TLS instead of handshaking per request.
func TestPooledHTTPClientKeepsPerHostConnections(t *testing.T) {
	client := NewPooledHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.MaxIdleConnsPerHost <= 0 || transport.MaxIdleConns <= 0 {
		t.Fatalf("pooling not configured: perHost=%d total=%d",
			transport.MaxIdleConnsPerHost, transport.MaxIdleConns)
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
