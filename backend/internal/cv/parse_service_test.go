package cv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/storage"
)

// fakeParser returns canned data (or an error) without touching the network.
type fakeParser struct {
	out ParsedCV
	err error
}

func (f fakeParser) Parse(context.Context, []byte, string, string) (ParsedCV, error) {
	return f.out, f.err
}

// fakeProfiles records the last ApplyParsedCV call.
type fakeProfiles struct {
	userID string
	got    ParsedCV
	called bool
	err    error
}

func (f *fakeProfiles) ApplyParsedCV(_ context.Context, userID string, p ParsedCV) error {
	f.called, f.userID, f.got = true, userID, p
	return f.err
}

func parseHarness(t *testing.T, parser Parser, profiles ProfileWriter) (*MemoryRepo, storage.ObjectStore, http.Handler) {
	t.Helper()
	repo := NewMemoryRepo()
	enc, err := storage.NewEncryptedStore(storage.NewMemoryStore(), bytes.Repeat([]byte{0x2a}, 32))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	now := func() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) }
	svc := NewService(repo, enc, now).WithParsing(parser, profiles)
	h := withUser(svc.Routes(), auth.User{ID: "user-1", Email: "u@example.com", Verified: true})
	return repo, enc, h
}

func seedFile(t *testing.T, repo *MemoryRepo, store storage.ObjectStore, userID string) File {
	t.Helper()
	key := "cv/" + userID + "/x.pdf"
	if err := store.Put(context.Background(), key, []byte("%PDF-1.7 data")); err != nil {
		t.Fatalf("seed put: %v", err)
	}
	rec, err := repo.CreateFile(context.Background(), File{
		UserID: userID, ObjectKey: key, Filename: "cv.pdf",
		ContentType: contentTypePDF, SizeBytes: 13, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("seed record: %v", err)
	}
	return rec
}

// AC-CV-2: parsing a stored CV returns structured data and writes it into the
// user's profile (which stays unconfirmed).
func TestParseEndpointAppliesToProfile(t *testing.T) {
	want := ParsedCV{
		FullName: "Budi Santoso",
		Skills:   []string{"Go", "React"},
	}
	profiles := &fakeProfiles{}
	repo, store, handler := parseHarness(t, fakeParser{out: want}, profiles)
	rec := seedFile(t, repo, store, "user-1")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cv/"+rec.ID+"/parse", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got ParsedCV
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.FullName != want.FullName {
		t.Errorf("full_name = %q, want %q", got.FullName, want.FullName)
	}
	if !profiles.called || profiles.userID != "user-1" {
		t.Fatalf("ApplyParsedCV not called for user: %+v", profiles)
	}
}

// A file owned by another user must not be parseable (no ownership leak).
func TestParseEndpointForbidsOtherUsersFile(t *testing.T) {
	profiles := &fakeProfiles{}
	repo, store, handler := parseHarness(t, fakeParser{}, profiles)
	rec := seedFile(t, repo, store, "someone-else")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cv/"+rec.ID+"/parse", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if profiles.called {
		t.Fatal("ApplyParsedCV should not be called for another user's file")
	}
}

func TestParseEndpointUnavailable(t *testing.T) {
	repo, store, handler := parseHarness(t, fakeParser{err: ErrParserUnavailable}, &fakeProfiles{})
	rec := seedFile(t, repo, store, "user-1")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cv/"+rec.ID+"/parse", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestParseEndpointUpstreamError(t *testing.T) {
	repo, store, handler := parseHarness(t, fakeParser{err: errors.New("boom")}, &fakeProfiles{})
	rec := seedFile(t, repo, store, "user-1")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cv/"+rec.ID+"/parse", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
}

// When no parser is configured the route is not even registered.
func TestParseRouteAbsentWithoutParser(t *testing.T) {
	repo := NewMemoryRepo()
	enc, _ := storage.NewEncryptedStore(storage.NewMemoryStore(), bytes.Repeat([]byte{0x2a}, 32))
	svc := NewService(repo, enc, nil)
	handler := withUser(svc.Routes(), auth.User{ID: "user-1", Verified: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cv/abc/parse", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (route unregistered)", rr.Code)
	}
}
