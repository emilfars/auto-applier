package cv

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/storage"
	"github.com/google/uuid"
)

type harness struct {
	repo    *MemoryRepo
	mem     *storage.MemoryStore
	handler http.Handler
}

type contentRepo struct {
	file File
	err  error
}

func (r contentRepo) CreateFile(context.Context, File) (File, error) {
	return File{}, r.err
}

func (r contentRepo) FileByID(context.Context, string) (File, error) {
	if r.err != nil {
		return File{}, r.err
	}
	return r.file, nil
}

func (r contentRepo) FilesByUser(context.Context, string) ([]File, error) {
	return nil, r.err
}

func (r contentRepo) DeleteByUser(context.Context, string) error {
	return r.err
}

// newHarness wires the CV service over an encrypted store (so at-rest bytes are
// ciphertext) and injects a fixed authenticated user via auth middleware
// substitute.
func newHarness(t *testing.T) *harness {
	t.Helper()
	repo := NewMemoryRepo()
	mem := storage.NewMemoryStore()
	key := bytes.Repeat([]byte{0x2a}, 32)
	enc, err := storage.NewEncryptedStore(mem, key)
	if err != nil {
		t.Fatalf("encrypted store: %v", err)
	}
	now := func() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) }
	svc := NewService(repo, enc, now)
	// Inject a verified user into context, mimicking auth.RequireVerified.
	h := &harness{repo: repo, mem: mem}
	h.handler = withUser(svc.Routes(), auth.User{ID: "user-1", Email: "u@example.com", Verified: true})
	return h
}

func withUser(next http.Handler, u auth.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(auth.ContextWithUser(r.Context(), u)))
	})
}

// upload builds a multipart request with a single file part and returns the
// recorder.
func (h *harness) upload(t *testing.T, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/cv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func pdfBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte("%PDF-1.7\n"))
	return b
}

func docxBytes() []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte("<xml/>"))
	}
	_ = w.Close()
	return b.Bytes()
}

// AC-CV-1: accept PDF/DOCX ≤5MB; reject other types (415) and >5MB (413).
func TestAC_CV_1_UploadMatrix(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		content  []byte
		want     int
	}{
		{"pdf ok", "resume.pdf", pdfBytes(1024), http.StatusCreated},
		{"docx ok", "resume.docx", docxBytes(), http.StatusCreated},
		{"zip renamed docx rejected", "fake.docx", []byte{0x50, 0x4B, 0x03, 0x04}, http.StatusUnsupportedMediaType},
		{"png rejected", "photo.png", []byte{0x89, 0x50, 0x4E, 0x47}, http.StatusUnsupportedMediaType},
		{"spoofed pdf ext", "fake.pdf", []byte("not really a pdf"), http.StatusUnsupportedMediaType},
		{"too large", "big.pdf", pdfBytes(int(MaxUploadBytes) + 1), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			rec := h.upload(t, tc.filename, tc.content)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// AC-CV-1b: stored CV object is encrypted at rest (ciphertext != plaintext).
func TestAC_CV_1b_EncryptedAtRest(t *testing.T) {
	h := newHarness(t)
	plaintext := append([]byte("%PDF-1.7\n"), []byte("SENSITIVE-CV-CONTENT")...)
	rec := h.upload(t, "resume.pdf", plaintext)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: got %d, want 201 (%s)", rec.Code, rec.Body.String())
	}

	files, _ := h.repo.FilesByUser(context.Background(), "user-1")
	if len(files) != 1 {
		t.Fatalf("expected 1 file record, got %d", len(files))
	}
	raw, err := h.mem.Get(context.Background(), files[0].ObjectKey)
	if err != nil {
		t.Fatalf("raw get: %v", err)
	}
	if bytes.Equal(raw, plaintext) {
		t.Fatal("stored bytes equal plaintext — not encrypted at rest")
	}
	if bytes.Contains(raw, []byte("SENSITIVE-CV-CONTENT")) {
		t.Fatal("plaintext content leaked into at-rest bytes")
	}
}

func TestUploadRequiresAuth(t *testing.T) {
	repo := NewMemoryRepo()
	svc := NewService(repo, storage.NewMemoryStore(), nil)
	req := httptest.NewRequest(http.MethodPost, "/cv", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	svc.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: got %d, want 401", rec.Code)
	}
}

func TestListReturnsUserFiles(t *testing.T) {
	h := newHarness(t)
	h.upload(t, "a.pdf", pdfBytes(512))
	h.upload(t, "b.docx", docxBytes())

	req := httptest.NewRequest(http.MethodGet, "/cv", nil)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: got %d, want 200", rec.Code)
	}

	var body struct {
		Files []fileResp `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(body.Files))
	}
}

func TestContentPreservesUnexpectedRepoError(t *testing.T) {
	repoErr := errors.New("database unavailable")
	svc := NewService(contentRepo{err: repoErr}, storage.NewMemoryStore(), nil)

	_, _, err := svc.Content(context.Background(), "user-1", "cv-1")
	if !errors.Is(err, repoErr) {
		t.Fatalf("got %v, want wrapped repo error", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("unexpected repo error was masked as not found: %v", err)
	}
}

func TestContentMasksCrossUserAccess(t *testing.T) {
	repo := contentRepo{file: File{
		ID: "cv-1", UserID: "user-1", ObjectKey: "cv/user-1/resume.pdf",
	}}
	svc := NewService(repo, storage.NewMemoryStore(), nil)

	_, _, err := svc.Content(context.Background(), "user-2", "cv-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestMemoryRepoCreatesUUIDFileIDs(t *testing.T) {
	file, err := NewMemoryRepo().CreateFile(context.Background(), File{UserID: "user-1"})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := uuid.Parse(file.ID); err != nil {
		t.Fatalf("memory file id = %q, want UUID: %v", file.ID, err)
	}
}

func TestAC_CV_5_MultipleVersionsAndPrimarySelection(t *testing.T) {
	repo := NewMemoryRepo()
	first, err := repo.CreateFile(context.Background(), File{UserID: "user-1", Filename: "base.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateFile(context.Background(), File{UserID: "user-1", Filename: "tech.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsPrimary || second.IsPrimary {
		t.Fatalf("initial primary versions = %+v, %+v", first, second)
	}
	primary := true
	if _, err := repo.UpdateVersion(context.Background(), "user-1", second.ID, "Technical CV", &primary); err != nil {
		t.Fatal(err)
	}
	files, err := repo.FilesByUser(context.Background(), "user-1")
	if err != nil || len(files) != 2 || !files[0].IsPrimary || files[0].Label != "Technical CV" {
		t.Fatalf("versions = %+v, err=%v", files, err)
	}
}
