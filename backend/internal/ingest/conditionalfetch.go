package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// maxFetchBody bounds a single listing payload so a hostile or broken source
// cannot exhaust memory.
const maxFetchBody = 16 << 20

// conditionalStore caches one source's last successful validators (ETag /
// Last-Modified) and decoded jobs. When the server answers 304 Not Modified it
// replays the cached jobs instead of returning nothing: the runner must
// re-touch (upsert) every live listing each pass or the 48h staleness sweep
// will expire them (SCR-4). Conditional requests therefore save bandwidth
// without becoming a "skip known listings" shortcut. The cache is in-memory and
// per source instance; a restart simply performs an unconditional fetch.
type conditionalStore struct {
	mu           sync.Mutex
	etag         string
	lastModified string
	cached       []RawJob
	hasCache     bool
}

// fetch performs a conditional GET and decodes the body. 304 replays the cache.
func (s *conditionalStore) fetch(
	ctx context.Context,
	client *http.Client,
	id string,
	build func() (*http.Request, error),
	decode func([]byte) ([]RawJob, error),
) ([]RawJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, err := build()
	if err != nil {
		return nil, fmt.Errorf("ingest: build request for %s: %w", id, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoApplier/1.0 (+https://autoapplier.id)")
	if s.etag != "" {
		req.Header.Set("If-None-Match", s.etag)
	}
	if s.lastModified != "" {
		req.Header.Set("If-Modified-Since", s.lastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingest: fetch %s: %w", id, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if !s.hasCache {
			return nil, fmt.Errorf("ingest: %s returned 304 with no cached payload", id)
		}
		return cloneRawJobs(s.cached), nil
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBody))
		if err != nil {
			return nil, fmt.Errorf("ingest: read %s: %w", id, err)
		}
		jobs, err := decode(body)
		if err != nil {
			return nil, err
		}
		s.etag = resp.Header.Get("ETag")
		s.lastModified = resp.Header.Get("Last-Modified")
		s.cached = cloneRawJobs(jobs)
		s.hasCache = true
		return jobs, nil
	default:
		return nil, fmt.Errorf("ingest: %s returned status %d", id, resp.StatusCode)
	}
}

// cloneRawJobs deep-copies the mutable Requirements slice so a caller cannot
// mutate the cache.
func cloneRawJobs(in []RawJob) []RawJob {
	if in == nil {
		return nil
	}
	out := make([]RawJob, len(in))
	for i, j := range in {
		out[i] = j
		if j.Requirements != nil {
			reqs := make([]string, len(j.Requirements))
			copy(reqs, j.Requirements)
			out[i].Requirements = reqs
		}
	}
	return out
}
