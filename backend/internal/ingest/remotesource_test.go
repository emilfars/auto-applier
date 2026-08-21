package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteSourcesNormalizeWorldwideJobs(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		build   func(string, string, int, *http.Client) Source
		want    string
	}{
		{
			name: "remotive",
			payload: `{"jobs":[
  {"title":"Remote Go Engineer","company_name":"Remote Co","url":"https://jobs.example.test/remotive/1","candidate_required_location":"Worldwide","salary":"","publication_date":"2026-06-01T10:00:00","description":"<p>Go</p>"},
  {"title":"Remote Product Lead","company_name":"Remote Co","url":"https://jobs.example.test/remotive/2","candidate_required_location":"Worldwide","salary":"Rp 20.000.000 - Rp 25.000.000","publication_date":"2026-06-01T11:00:00","description":"Product"}
]}`,
			build: func(id, endpoint string, limit int, client *http.Client) Source {
				return NewRemotiveSourceWithURL(id, endpoint, limit, client)
			},
			want: "Remote Go Engineer",
		},
		{
			name:    "jobicy",
			payload: `{"jobs":[{"jobTitle":"Remote Data Engineer","companyName":"Jobicy Co","url":"https://jobs.example.test/jobicy/1","jobGeo":"Anywhere","jobType":["Full-Time"],"pubDate":"2026-06-01T10:00:00Z","jobDescription":"<p>Data</p>","salaryMin":3000,"salaryMax":5000,"salaryCurrency":"USD"}]}`,
			build: func(id, endpoint string, limit int, client *http.Client) Source {
				return NewJobicySourceWithURL(id, endpoint, limit, client)
			},
			want: "Remote Data Engineer",
		},
		{
			name:    "remoteok",
			payload: `[{"legal":"Attribution required"},{"position":"Remote QA Engineer","company":"RemoteOK Co","url":"https://jobs.example.test/remoteok/1","date":"2026-06-01T10:00:00Z","tags":["qa"]}]`,
			build: func(id, endpoint string, limit int, client *http.Client) Source {
				return NewRemoteOKSourceWithURL(id, endpoint, limit, client)
			},
			want: "Remote QA Engineer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.payload))
			}))
			t.Cleanup(srv.Close)

			source := tc.build("remote-"+tc.name, srv.URL, 10, srv.Client())
			if source.Tier() != Tier2 {
				t.Fatalf("tier = %d, want Tier 2", source.Tier())
			}
			store := NewMemoryStore()
			registry := NewRegistry()
			if err := registry.Register(source); err != nil {
				t.Fatal(err)
			}
			report := NewRunner(registry, store, nil, 0, 0).RunOnce(context.Background())
			if report.TotalCreated() == 0 {
				t.Fatalf("remote listing was not persisted: %+v", report)
			}
			for _, job := range store.All() {
				if !job.Remote || job.Location != "Remote" {
					t.Fatalf("remote source did not normalize Remote location: %+v", job)
				}
				if job.SalaryStatedMin != nil && tc.name != "remotive" {
					t.Fatalf("foreign salary was retained: %+v", job)
				}
			}
			if _, found := findJob(store.All(), tc.want); !found {
				t.Fatalf("missing %q in %+v", tc.want, store.All())
			}
		})
	}
}

func findJob(jobs []Job, title string) (Job, bool) {
	for _, job := range jobs {
		if job.Title == title {
			return job, true
		}
	}
	return Job{}, false
}
