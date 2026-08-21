package m5

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/profile"
)

type testJobs struct{ jobs []ingest.Job }

func (j testJobs) ActiveJobs(context.Context) ([]ingest.Job, error) { return j.jobs, nil }

type testMailer struct{ to, subject, body string }

func (m *testMailer) Send(_ context.Context, to, subject, body string) error {
	m.to, m.subject, m.body = to, subject, body
	return nil
}

func testUserContext(req *http.Request) *http.Request {
	return req.WithContext(auth.ContextWithUser(req.Context(), auth.User{ID: "user-1", Email: "u@example.com"}))
}

func TestAC_SCR_5_SalaryEstimateAndAC_SCR_6_Tags(t *testing.T) {
	job := ingest.Job{Title: "Senior Backend Engineer", Location: "Jakarta", Requirements: []string{"Go", "PostgreSQL", "3+ years"}}
	estimate, ok := EstimateSalary(job)
	if !ok || estimate.Min >= estimate.Max || estimate.Confidence == "" {
		t.Fatalf("estimate = %+v, ok=%v", estimate, ok)
	}
	if got := ExtractRequirementTags(job.Requirements); len(got) != 2 || got[0] != "Go" || got[1] != "PostgreSQL" {
		t.Fatalf("tags = %v", got)
	}
	p := profile.Profile{Skills: json.RawMessage(`["golang", "Docker"]`)}
	if score, ok := MatchScore(job, p); !ok || score != 50 {
		t.Fatalf("match score = %d, ok=%v", score, ok)
	}
	if _, ok := EstimateSalary(ingest.Job{SalaryStatedMin: ptr(1)}); ok {
		t.Fatal("stated salary must not receive an estimate")
	}
}

func ptr(value int64) *int64 { return &value }

func TestAC_FEED_6_AC_APP_6_AC_APP_7_Routes(t *testing.T) {
	store := NewMemoryStore(time.Now)
	svc := NewService(store, testJobs{}, nil, nil, nil, time.Now)
	routes := svc.Routes()

	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rr := httptest.NewRecorder()
		routes.ServeHTTP(rr, testUserContext(req))
		return rr
	}

	state := request(http.MethodPost, "/jobs/job-1/applied", "")
	if state.Code != http.StatusOK || !strings.Contains(state.Body.String(), "already_applied") {
		t.Fatalf("state: %d %s", state.Code, state.Body.String())
	}
	snippet := request(http.MethodPost, "/snippets", `{"name":"intro","body":"Hi {name}, I am applying for {role} at {company}."}`)
	if snippet.Code != http.StatusCreated {
		t.Fatalf("create snippet: %d %s", snippet.Code, snippet.Body.String())
	}
	var created Snippet
	if err := json.Unmarshal(snippet.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	render := request(http.MethodPost, "/snippets/"+created.ID+"/render", `{"name":"Dina","role":"Engineer","company":"Acme"}`)
	if render.Code != http.StatusOK || !strings.Contains(render.Body.String(), "Dina") || !strings.Contains(render.Body.String(), "Acme") {
		t.Fatalf("render: %d %s", render.Code, render.Body.String())
	}
	app := request(http.MethodPost, "/applications", `{"job_id":"job-1"}`)
	if app.Code != http.StatusCreated {
		t.Fatalf("create application: %d %s", app.Code, app.Body.String())
	}
	var createdApp Application
	if err := json.Unmarshal(app.Body.Bytes(), &createdApp); err != nil {
		t.Fatal(err)
	}
	confirmed := request(http.MethodPost, "/applications/"+createdApp.ID+"/confirm", "")
	if confirmed.Code != http.StatusOK || !strings.Contains(confirmed.Body.String(), `"status":"submitted"`) {
		t.Fatalf("confirm application: %d %s", confirmed.Code, confirmed.Body.String())
	}
}

func TestAC_FEED_5_SavedFilterAlertsOnce(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	posted := now.Add(time.Minute)
	jobs := testJobs{jobs: []ingest.Job{{Title: "Go Engineer", Company: "Acme", Requirements: []string{"Go"}, PostedAt: &posted}}}
	mailer := &testMailer{}
	users := auth.NewMemoryRepo()
	user, err := users.CreateUser(context.Background(), "u@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.SetVerified(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}
	filter, err := store.CreateSavedFilter(context.Background(), SavedFilter{UserID: user.ID, Name: "Go roles", Query: ingest.JobQuery{Skills: []string{"Go"}}})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(store, jobs, users, nil, mailer, func() time.Time { return now.Add(2 * time.Minute) })
	count, err := svc.NotifyNewMatches(context.Background(), user.ID)
	if err != nil || count != 1 || mailer.to != user.Email {
		t.Fatalf("notify = %d, %v, mail=%+v", count, err, mailer)
	}
	count, err = svc.NotifyNewMatches(context.Background(), user.ID)
	if err != nil || count != 0 {
		t.Fatalf("duplicate notify = %d, %v", count, err)
	}
	if _, err := store.GetSavedFilter(context.Background(), user.ID, filter.ID); err != nil {
		t.Fatal(err)
	}
}
