// Command api starts the Auto Applier backend HTTP server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auto-applier/backend/internal/api"
	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// With DATABASE_URL set, auth persists to Postgres (migrations applied at
	// startup). Without it, an in-memory repo backs local/dev runs.
	authRepo, closeRepo := newAuthRepo()
	defer closeRepo()
	authSvc := auth.NewService(authRepo, auth.Config{})

	// CV uploads are stored encrypted at rest (AC-CV-1b) and gated behind a
	// verified session. Object-store backend is in-memory until S3 lands.
	cvSvc := cv.NewService(cv.NewMemoryRepo(), newCVStore(), time.Now)
	gatedCV := authSvc.RequireVerified(cvSvc.Routes())

	srv := &http.Server{
		Addr: addr,
		Handler: api.NewRouter(authSvc.Routes(),
			api.Mount{Pattern: "/cv", Handler: gatedCV},
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("api listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("api stopped")
}

// newAuthRepo selects the auth persistence backend. When DATABASE_URL is set it
// connects to Postgres, applies pending migrations, and returns a pgx-backed
// repo; otherwise it falls back to the in-memory repo for local development.
func newAuthRepo() (auth.Repo, func()) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Println("DATABASE_URL not set; using in-memory auth repo")
		return auth.NewMemoryRepo(), func() {}
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	applied, err := db.Apply(ctx, pool)
	if err != nil {
		pool.Close()
		log.Fatalf("apply migrations: %v", err)
	}
	log.Printf("database ready (%d migration(s) applied)", applied)
	return auth.NewPgxRepo(pool), pool.Close
}

// newCVStore builds the CV object store. CV bytes are always encrypted at rest
// (AC-CV-1b): CV_ENCRYPTION_KEY (64 hex chars = 32 bytes) is used when set,
// otherwise an ephemeral key is generated for local dev (data does not survive
// a restart, which is acceptable for the in-memory backend).
func newCVStore() storage.ObjectStore {
	key := decodeCVKey()
	enc, err := storage.NewEncryptedStore(storage.NewMemoryStore(), key)
	if err != nil {
		log.Fatalf("cv store: %v", err)
	}
	return enc
}

func decodeCVKey() []byte {
	if hexKey := os.Getenv("CV_ENCRYPTION_KEY"); hexKey != "" {
		key, err := hex.DecodeString(hexKey)
		if err != nil || len(key) != 32 {
			log.Fatalf("CV_ENCRYPTION_KEY must be 64 hex chars (32 bytes)")
		}
		return key
	}
	log.Println("CV_ENCRYPTION_KEY not set; generating an ephemeral dev key")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("generate cv key: %v", err)
	}
	return key
}
