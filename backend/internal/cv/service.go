package cv

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/storage"
)

// MaxUploadBytes is the CV size limit (AC-CV-1). Matches the cv_files.size_bytes
// CHECK constraint in the schema.
const MaxUploadBytes int64 = 5 << 20 // 5 MiB

const (
	contentTypePDF  = "application/pdf"
	contentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// magic byte prefixes used to sniff the real file type, so a caller cannot pass
// an arbitrary blob with a spoofed Content-Type header.
var (
	magicPDF = []byte("%PDF")
	magicZIP = []byte{0x50, 0x4B, 0x03, 0x04} // DOCX is a ZIP container
)

// Service exposes the CV upload API over a Repo and an encrypted ObjectStore.
type Service struct {
	repo  Repo
	store storage.ObjectStore
	now   func() time.Time
}

// NewService builds a CV service. store should be an encrypted store in
// production so CV bytes are encrypted at rest (AC-CV-1b).
func NewService(repo Repo, store storage.ObjectStore, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, store: store, now: now}
}

// Routes returns the CV HTTP handler. Callers must wrap it with auth
// middleware (RequireVerified) so an authenticated user is in context.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cv", s.handleUpload)
	mux.HandleFunc("GET /cv", s.handleList)
	return mux
}

type fileResp struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   string `json:"created_at"`
}

func (s *Service) handleUpload(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Cap the whole request body; leave headroom for multipart framing so the
	// per-file check below produces the precise 413 for oversized files.
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeErr(w, http.StatusRequestEntityTooLarge, "file exceeds 5MB limit")
			return
		}
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	// Read up to the limit + 1 byte to detect oversize precisely.
	data, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read error")
		return
	}
	if int64(len(data)) > MaxUploadBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file exceeds 5MB limit")
		return
	}

	contentType, ok := detectType(header.Filename, data)
	if !ok {
		writeErr(w, http.StatusUnsupportedMediaType, "only PDF and DOCX are supported")
		return
	}

	key, err := objectKey(u.ID, header.Filename)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "key error")
		return
	}
	if err := s.store.Put(r.Context(), key, data); err != nil {
		writeErr(w, http.StatusInternalServerError, "store error")
		return
	}

	rec, err := s.repo.CreateFile(r.Context(), File{
		UserID:      u.ID,
		ObjectKey:   key,
		Filename:    header.Filename,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		CreatedAt:   s.now().UTC(),
	})
	if err != nil {
		_ = s.store.Delete(r.Context(), key)
		writeErr(w, http.StatusInternalServerError, "persist error")
		return
	}
	writeJSON(w, http.StatusCreated, toResp(rec))
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	files, err := s.repo.FilesByUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list error")
		return
	}
	out := make([]fileResp, 0, len(files))
	for _, f := range files {
		out = append(out, toResp(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

// detectType validates the upload by both extension and magic bytes, returning
// the canonical content type. A mismatch (e.g. a spoofed header) fails closed.
func detectType(filename string, data []byte) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".pdf" && bytes.HasPrefix(data, magicPDF):
		return contentTypePDF, true
	case ext == ".docx" && bytes.HasPrefix(data, magicZIP):
		return contentTypeDOCX, true
	default:
		return "", false
	}
}

func objectKey(userID, filename string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cv/" + userID + "/" + hex.EncodeToString(buf) + strings.ToLower(filepath.Ext(filename)), nil
}

func toResp(f File) fileResp {
	return fileResp{
		ID:          f.ID,
		Filename:    f.Filename,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		CreatedAt:   f.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
