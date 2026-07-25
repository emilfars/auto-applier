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
	"strconv"
	"syscall"
	"time"

	"github.com/auto-applier/backend/internal/account"
	"github.com/auto-applier/backend/internal/api"
	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/cv"
	"github.com/auto-applier/backend/internal/db"
	"github.com/auto-applier/backend/internal/feed"
	"github.com/auto-applier/backend/internal/ingest"
	"github.com/auto-applier/backend/internal/profile"
	"github.com/auto-applier/backend/internal/queue"
	"github.com/auto-applier/backend/internal/seed"
	"github.com/auto-applier/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// With DATABASE_URL set, data persists to Postgres (migrations applied at
	// startup) and the pool is shared across repos. Without it, in-memory repos
	// back local/dev runs.
	pool, closeDB := openDB()
	defer closeDB()

	authRepo := newAuthRepo(pool)
	authSvc := auth.NewService(authRepo, auth.Config{
		TrustedProxyHops: intEnv("TRUSTED_PROXY_HOPS", 0),
		DevExposeTokens:  boolEnv("AUTH_DEV_EXPOSE_TOKENS", false),
	})

	// CV uploads are stored encrypted at rest (AC-CV-1b) and gated behind a
	// verified session. Object-store backend is in-memory until S3 lands.
	// Parsing (AC-CV-2) uses a hosted resume-parse API when CV_PARSER_URL is
	// set; otherwise it degrades to 503 (upload/edit still work fully).
	// Profile edit + confirm-before-apply gate (AC-CV-3/4/5).
	profileRepo := newProfileRepo(pool)
	profileSvc := profile.NewService(profileRepo, time.Now)
	gatedProfile := authSvc.RequireVerified(profileSvc.Routes())

	cvSvc := cv.NewService(newCVRepo(pool), newCVStore(), time.Now).
		WithParsing(newCVParser(), profileSvc)
	gatedCV := authSvc.RequireVerified(cvSvc.Routes())

	// Data-rights endpoints (AC-AUTH-5 / UU PDP): export all user data and
	// delete the account with every piece of PII it owns.
	accountSvc := account.NewService(authRepo, profileRepo, cvSvc)
	gatedAccount := authSvc.RequireVerified(accountSvc.Routes())

	// Public job feed (FEED-1..3): browsable before signup, stated-pay only.
	// Postgres-backed when DATABASE_URL is set (jobs persist across restarts and
	// are populated by the seed/ingestion); otherwise an in-memory store with a
	// synthetic demo seed backs local dev.
	jobStore, pgxJobStore := newJobStore(pool)
	feedSvc := feed.NewService(jobStore)

	// Background ingestion + staleness sweep via River (Postgres only). Falls
	// back to the boot-time synthetic seed when no DB/sources are configured.
	stopQueue := startIngestQueue(pool, pgxJobStore)
	defer stopQueue()

	srv := &http.Server{
		Addr: addr,
		Handler: api.RequireHTTPS(api.NewRouter(authSvc.Routes(),
			api.Mount{Pattern: "/cv", Handler: gatedCV},
			api.Mount{Pattern: "/profile", Handler: gatedProfile},
			api.Mount{Pattern: "/profile/", Handler: gatedProfile},
			api.Mount{Pattern: "/account", Handler: gatedAccount},
			api.Mount{Pattern: "/account/", Handler: gatedAccount},
			api.Mount{Pattern: "/feed", Handler: feedSvc.Routes()},
		), api.SecurityConfig{EnforceHTTPS: boolEnv("ENFORCE_HTTPS", false)}),
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

// newJobStore returns the feed's job provider and, when Postgres backs it, the
// concrete pgx store (so the ingestion queue can also sweep stale listings). In
// in-memory mode the second return is nil and a synthetic demo seed is loaded.
func newJobStore(pool *pgxpool.Pool) (feed.Provider, *ingest.PgxStore) {
	if pool == nil {
		ms := ingest.NewMemoryStore()
		seedFeed(ms)
		return ms, nil
	}
	log.Println("using Postgres-backed job store")
	store := ingest.NewPgxStore(pool)
	return store, store
}

// startIngestQueue starts the River-backed periodic ingestion + staleness
// sweep. It is a no-op (returning a no-op stop) when Postgres is not configured
// or no live sources are enabled, so local dev without a DB still runs. A start
// failure is logged and degraded — the API keeps serving the existing feed.
func startIngestQueue(pool *pgxpool.Pool, store *ingest.PgxStore) func() {
	noop := func() {}
	if pool == nil || store == nil {
		return noop
	}
	reg := ingest.BuildRegistry(ingest.SourceConfig{
		KalibrrLimit: intEnv("KALIBRR_LIMIT", 25),
		JoobleAPIKey: os.Getenv("JOOBLE_API_KEY"),
	})
	if len(reg.Sources()) == 0 {
		log.Println("no live sources enabled; skipping background ingestion")
		return noop
	}
	runner := ingest.NewRunner(reg, store, time.Now, 0, 0)
	client, err := queue.Start(context.Background(), pool, runner, store, queue.Config{
		IngestInterval: durEnv("INGEST_INTERVAL", 6*time.Hour),
		SweepInterval:  durEnv("SWEEP_INTERVAL", time.Hour),
		StaleAfter:     durEnv("STALE_AFTER", 14*24*time.Hour),
	})
	if err != nil {
		log.Printf("river: %v (continuing without background ingestion)", err)
		return noop
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Stop(ctx); err != nil {
			log.Printf("river stop: %v", err)
		}
	}
}

// seedFeed populates the in-memory job store with deterministic synthetic
// Jabodetabek listings so the public feed is demoable without a live scraper
// run. FEED_SEED_COUNT sets the volume (0 disables). Listings carry
// employer-stated pay only (locked decision) — the generator never fabricates
// an estimated salary.
func seedFeed(store *ingest.MemoryStore) {
	count := intEnv("FEED_SEED_COUNT", 200)
	if count <= 0 {
		return
	}
	n, err := seed.Seed(context.Background(), store, count, 1, time.Now())
	if err != nil {
		log.Printf("seed feed: %v", err)
		return
	}
	log.Printf("seeded %d demo listing(s) into feed", n)
}

// openDB opens a shared Postgres pool and applies migrations when DATABASE_URL
// is set, returning (nil, no-op) otherwise so callers fall back to in-memory
// repos for local development.
func openDB() (*pgxpool.Pool, func()) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Println("DATABASE_URL not set; using in-memory repos")
		return nil, func() {}
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
	return pool, pool.Close
}

// newAuthRepo returns a pgx-backed auth repo when a pool is available, else the
// in-memory repo.
func newAuthRepo(pool *pgxpool.Pool) auth.Repo {
	if pool == nil {
		return auth.NewMemoryRepo()
	}
	return auth.NewPgxRepo(pool)
}

// newProfileRepo returns a pgx-backed profile repo when a pool is available,
// else the in-memory repo.
func newProfileRepo(pool *pgxpool.Pool) profile.Repo {
	if pool == nil {
		return profile.NewMemoryRepo()
	}
	return profile.NewPgxRepo(pool)
}

// newCVRepo returns a pgx-backed CV metadata repo when a pool is available, else
// the in-memory repo. CV bytes always live in the encrypted object store; only
// metadata (object-key references) go to Postgres.
func newCVRepo(pool *pgxpool.Pool) cv.Repo {
	if pool == nil {
		return cv.NewMemoryRepo()
	}
	return cv.NewPgxRepo(pool)
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

// intEnv reads an integer environment variable, falling back to def when unset
// or unparseable.
func intEnv(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("%s=%q is not an integer; using %d", name, v, def)
	}
	return def
}

// boolEnv reads a boolean environment variable, falling back to def when unset
// or unparseable.
func boolEnv(name string, def bool) bool {
	if v := os.Getenv(name); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		log.Printf("%s=%q is not a boolean; using %v", name, v, def)
	}
	return def
}

// durEnv reads a Go duration environment variable (e.g. "6h", "30m"), falling
// back to def when unset or unparseable.
func durEnv(name string, def time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("%s=%q is not a duration; using %s", name, v, def)
	}
	return def
}

// newCVParser returns the hosted CV parser when CV_PARSER_URL is set, else a
// disabled parser so the parse endpoint responds 503 without breaking upload or
// profile editing.
func newCVParser() cv.Parser {
	url := os.Getenv("CV_PARSER_URL")
	if url == "" {
		log.Println("CV_PARSER_URL not set; CV parsing disabled")
		return cv.NewDisabledParser()
	}
	return cv.NewHostedParser(url, os.Getenv("CV_PARSER_API_KEY"), nil)
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
