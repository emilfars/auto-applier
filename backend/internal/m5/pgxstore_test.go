package m5

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newM5PgxStore(t *testing.T) (*PgxStore, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping M5 Postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := db.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	var userID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email) VALUES ($1) RETURNING id::text`, fmt.Sprintf("m5-%d@example.com", time.Now().UnixNano())).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })
	return NewPgxStore(pool), userID
}

func TestAC_M5_PgxStoreRoundTrip(t *testing.T) {
	store, userID := newM5PgxStore(t)
	ctx := context.Background()
	if _, err := store.db.Exec(ctx, `INSERT INTO jobs (source, source_url, dedup_key, title, company, requirements) VALUES ('fixture', 'https://example.test/m5', 'm5-job', 'Go Engineer', 'Acme', '["Go"]')`); err != nil {
		t.Fatalf("create job: %v", err)
	}
	filter, err := store.CreateSavedFilter(ctx, SavedFilter{UserID: userID, Name: "Go roles", Query: ingest.JobQuery{Skills: []string{"Go"}}})
	if err != nil || filter.ID == "" {
		t.Fatalf("create filter: %+v, %v", filter, err)
	}
	state, err := store.SetJobState(ctx, userID, JobState{JobKey: "m5-job", AlreadyApplied: true})
	if err != nil || !state.AlreadyApplied {
		t.Fatalf("state: %+v, %v", state, err)
	}
	app, err := store.CreateApplication(ctx, Application{UserID: userID, JobKey: "m5-job", Status: StatusFormFilled})
	if err != nil || app.JobTitle != "Go Engineer" {
		t.Fatalf("application: %+v, %v", app, err)
	}
	snippet, err := store.CreateSnippet(ctx, Snippet{UserID: userID, Name: "intro", Body: "Hi {name}"})
	if err != nil || snippet.ID == "" {
		t.Fatalf("snippet: %+v, %v", snippet, err)
	}
	data, err := store.ExportUser(ctx, userID)
	if err != nil || len(data.SavedFilters) != 1 || len(data.JobStates) != 1 || len(data.Applications) != 1 || len(data.Snippets) != 1 {
		t.Fatalf("export: %+v, %v", data, err)
	}
}
