package db

import (
	"strings"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() error: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one migration")
	}

	for i, m := range migrations {
		if m.Version != i+1 {
			t.Errorf("migration[%d] version = %d, want %d", i, m.Version, i+1)
		}
		if strings.TrimSpace(m.Up) == "" {
			t.Errorf("version %d has empty up SQL", m.Version)
		}
		if strings.TrimSpace(m.Down) == "" {
			t.Errorf("version %d has empty down SQL", m.Version)
		}
	}
}

// AC-SCR-2b (schema side) + MVP salary rule: schema must not model estimated
// salary, and applications must never default to a submitted state.
func TestInitMigrationInvariants(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() error: %v", err)
	}

	up := migrations[0].Up

	mustContain := []string{
		"CREATE TABLE users",
		"CREATE TABLE profiles",
		"CREATE TABLE cv_files",
		"CREATE TABLE jobs",
		"CREATE TABLE applications",
		"confirmed",         // confirm-before-apply gate
		"salary_stated_min", // stated-salary only
	}
	for _, s := range mustContain {
		if !strings.Contains(up, s) {
			t.Errorf("init migration missing expected fragment %q", s)
		}
	}

	// MVP is stated-salary only: no estimated-salary columns.
	if strings.Contains(strings.ToLower(up), "salary_estimated") ||
		strings.Contains(strings.ToLower(up), "estimated_salary") {
		t.Error("init migration must not model estimated salary at MVP")
	}

	// Prime directive: default application status is never 'submitted'.
	if strings.Contains(up, "DEFAULT 'submitted'") {
		t.Error("applications must not default to 'submitted' — submission is always a human action")
	}
}

func TestJobProvenanceMigrationBackfillsOnlyGeneratorURLs(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() error: %v", err)
	}

	var up string
	for _, migration := range migrations {
		if migration.Version == 7 {
			up = migration.Up
			break
		}
	}
	if !strings.Contains(up, "synthetic BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Fatal("job provenance migration must add a non-null synthetic flag")
	}
	if !strings.Contains(up, "source_url LIKE 'https://example.test/%'") {
		t.Fatal("job provenance migration must backfill known generator URLs only")
	}
}

func TestProfileAppFieldsMigration(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() error: %v", err)
	}
	var up string
	for _, migration := range migrations {
		if migration.Version == 8 {
			up = migration.Up
			break
		}
	}
	for _, column := range []string{
		"linkedin_url", "github_url", "portfolio_url", "address", "city",
		"summary", "current_employer", "current_title", "highest_education",
	} {
		if !strings.Contains(up, column) {
			t.Errorf("profile app-fields migration missing %q", column)
		}
	}
}

func TestAC_M5_Migrations(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() error: %v", err)
	}
	checks := map[int][]string{
		9:  {"saved_filters", "job_user_states", "answer_snippets"},
		10: {"label", "is_primary", "cv_files_one_primary_per_user"},
	}
	for version, fragments := range checks {
		var up string
		for _, migration := range migrations {
			if migration.Version == version {
				up = migration.Up
				break
			}
		}
		for _, fragment := range fragments {
			if !strings.Contains(up, fragment) {
				t.Errorf("migration %d missing %q", version, fragment)
			}
		}
	}
}
