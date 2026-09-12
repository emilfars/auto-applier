// Command api starts the Auto Applier backend HTTP server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/auto-applier/backend/internal/ingestctl"
	"github.com/auto-applier/backend/internal/m5"
	"github.com/auto-applier/backend/internal/profile"
	"github.com/auto-applier/backend/internal/queue"
	"github.com/auto-applier/backend/internal/seed"
	"github.com/auto-applier/backend/internal/storage"
	"github.com/auto-applier/backend/internal/telemetry"
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

	trustedProxyHops := intEnv("TRUSTED_PROXY_HOPS", 0)
	authRepo := newAuthRepo(pool)
	mailer, err := newMailer()
	if err != nil {
		log.Fatalf("configure mailer: %v", err)
	}
	authSvc := auth.NewService(authRepo, auth.Config{
		TrustedProxyHops: trustedProxyHops,
		DevExposeTokens:  boolEnv("AUTH_DEV_EXPOSE_TOKENS", false),
		Mailer:           mailer,
	})

	// CV uploads are stored encrypted at rest (AC-CV-1b) and gated behind a
	// verified session.
	// Parsing (AC-CV-2) uses a hosted resume-parse API when CV_PARSER_URL is
	// set; otherwise it degrades to 503 (upload/edit still work fully).
	// Profile edit + confirm-before-apply gate (AC-CV-3/4/5).
	profileRepo := newProfileRepo(pool)
	profileSvc := profile.NewService(profileRepo, time.Now)

	cvStore, err := newCVStore(context.Background())
	if err != nil {
		log.Fatalf("cv store: %v", err)
	}
	cvSvc := cv.NewService(newCVRepo(pool), cvStore, time.Now).
		WithParsing(newCVParser(), profileSvc)
	profileSvc.WithCVSource(cvSvc)
	gatedProfile := authSvc.RequireVerified(profileSvc.Routes())
	gatedCV := authSvc.RequireVerified(cvSvc.Routes())

	// Data-rights endpoints (AC-AUTH-5 / UU PDP): export all user data and
	// delete the account with every piece of PII it owns.
	telemetrySvc := telemetry.NewService()

	// Public job feed: browsable before signup; M5 estimates are explicitly
	// labeled separately from stated pay.
	// Postgres-backed when DATABASE_URL is set (jobs persist across restarts and
	// are populated by the seed/ingestion); otherwise an in-memory store with a
	// synthetic demo seed backs local dev.
	jobStore, jobCounter, pgxJobStore := newJobStore(pool)
	m5Store := newM5Store(pool)
	accountSvc := account.NewService(authRepo, profileRepo, cvSvc).WithAdditionalStore(m5Store)
	gatedAccount := authSvc.RequireVerified(accountSvc.Routes())
	m5Svc := m5.NewService(m5Store, jobStore, authRepo, profileRepo, mailer, time.Now)
	feedSvc := feed.NewService(jobStore).WithPersonalization(profileRepo, m5Store)
	stopAlerts := startM5AlertLoop(m5Svc, mailer != nil, durEnv("ALERT_INTERVAL", 6*time.Hour))
	defer stopAlerts()

	// Background ingestion + staleness sweep via River (Postgres only).
	stopQueue, ingestRunner, err := startIngestQueue(pool, pgxJobStore)
	if err != nil {
		log.Fatalf("queue startup: %v", err)
	}
	defer stopQueue()

	// Backfill at boot only when the feed is below the launch target, and expose
	// an authenticated on-demand trigger. A healthy deployment must not re-fetch
	// every source on every restart.
	backfillTarget := intEnv("INGEST_BACKFILL_TARGET", seed.DefaultCount)
	ingestCtl := ingestctl.New(jobCounter, backfillTarget, func(ctx context.Context) {
		if ingestRunner == nil {
			return
		}
		rep := ingestRunner.RunOnce(ctx)
		log.Printf("ingest: %d new listing(s) across %d source(s)", rep.TotalCreated(), len(rep.Sources))
	}, durEnv("INGEST_TRIGGER_MIN_INTERVAL", time.Minute))
	if started, backfillErr := ingestCtl.StartBackfill(context.Background()); backfillErr != nil {
		log.Printf("startup backfill check: %v", backfillErr)
	} else if started {
		log.Printf("startup backfill started (active real listings below %d)", backfillTarget)
	}

	srv := &http.Server{
		Addr: addr,
		Handler: api.RequireHTTPS(api.NewRouter(registrationGate(authSvc.Routes(), jobCounter),
			api.Mount{Pattern: "/cv", Handler: gatedCV},
			api.Mount{Pattern: "/profile", Handler: gatedProfile},
			api.Mount{Pattern: "/profile/", Handler: gatedProfile},
			api.Mount{Pattern: "/account", Handler: gatedAccount},
			api.Mount{Pattern: "/account/", Handler: gatedAccount},
			api.Mount{Pattern: "/feed", Handler: authSvc.OptionalVerified(feedSvc.Routes())},
			api.Mount{Pattern: "/telemetry/", Handler: telemetrySvc.Routes()},
			api.Mount{Pattern: "/saved-filters", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/saved-filters/", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/jobs/", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/applications", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/applications/", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/snippets", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/snippets/", Handler: authSvc.RequireVerified(m5Svc.Routes())},
			api.Mount{Pattern: "/ingest/", Handler: ingestCtl.Handler(os.Getenv("INGEST_TRIGGER_TOKEN"))},
		), api.SecurityConfig{
			EnforceHTTPS:        boolEnv("ENFORCE_HTTPS", false),
			TrustForwardedProto: trustedProxyHops > 0,
		}),
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

func startM5AlertLoop(service *m5.Service, enabled bool, interval time.Duration) func() {
	if !enabled || interval <= 0 {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := service.NotifyAllMatches(ctx); err != nil {
					log.Printf("m5 alerts: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func newM5Store(pool *pgxpool.Pool) m5.Store {
	if pool == nil {
		return m5.NewMemoryStore(time.Now)
	}
	return m5.NewPgxStore(pool)
}

// newJobStore returns the feed provider, its real-active counter, and, when
// Postgres backs it, the concrete pgx store for the ingestion queue. In-memory
// mode loads a synthetic demo seed.
type realActiveJobCounter interface {
	RealActiveCount(context.Context) (int, error)
}

func newJobStore(pool *pgxpool.Pool) (feed.Provider, realActiveJobCounter, *ingest.PgxStore) {
	if pool == nil {
		ms := ingest.NewMemoryStore()
		seedFeed(ms)
		return ms, nil, nil
	}
	log.Println("using Postgres-backed job store")
	store := ingest.NewPgxStore(pool)
	return store, store, store
}

func registrationGate(next http.Handler, jobs realActiveJobCounter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/register" {
			next.ServeHTTP(w, r)
			return
		}
		if jobs == nil {
			if boolEnv("AUTH_DEV_ALLOW_IN_MEMORY_REGISTRATION", false) &&
				boolEnv("AUTH_DEV_EXPOSE_TOKENS", false) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w,
				fmt.Sprintf("registration unavailable until feed has at least %d real active job listings", seed.DefaultCount),
				http.StatusServiceUnavailable,
			)
			return
		}
		count, err := jobs.RealActiveCount(r.Context())
		if err != nil || count < seed.DefaultCount {
			http.Error(w,
				fmt.Sprintf("registration unavailable until feed has at least %d real active job listings", seed.DefaultCount),
				http.StatusServiceUnavailable,
			)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type queueClient interface {
	Stop(context.Context) error
}

type queueStarter func(context.Context, *pgxpool.Pool, *ingest.Runner, *ingest.PgxStore, queue.Config) (queueClient, error)

func startIngestQueue(pool *pgxpool.Pool, store *ingest.PgxStore) (func(), *ingest.Runner, error) {
	return startIngestQueueWith(pool, store, func(
		ctx context.Context,
		pool *pgxpool.Pool,
		runner *ingest.Runner,
		store *ingest.PgxStore,
		cfg queue.Config,
	) (queueClient, error) {
		return queue.Start(ctx, pool, runner, store, cfg)
	})
}

func startIngestQueueWith(pool *pgxpool.Pool, store *ingest.PgxStore, start queueStarter) (func(), *ingest.Runner, error) {
	noop := func() {}
	if pool == nil || store == nil {
		return noop, nil, nil
	}
	reg := ingest.BuildRegistry(ingest.SourceConfig{
		KalibrrLimit: intEnv("KALIBRR_LIMIT", seed.DefaultCount),
		JoobleAPIKey: os.Getenv("JOOBLE_API_KEY"),
		JoobleLimit:  intEnv("JOOBLE_LIMIT", seed.DefaultCount),
		// Jooble is lifetime-quota backfill only and is not part of the recurring
		// queue. The real seed command opts into it explicitly.
		JoobleBackfill: false,
		CareerjetAffid: os.Getenv("CAREERJET_AFFID"),
		CareerjetLimit: intEnv("CAREERJET_LIMIT", seed.DefaultCount),
		ATSEnabled:     boolEnv("ATS_PUBLIC_ENABLED", true),
		RemoteEnabled:  boolEnv("REMOTE_SOURCES_ENABLED", true),
		RemoteLimit:    intEnv("REMOTE_LIMIT", 100),
	})
	runner := ingest.NewRunner(reg, store, time.Now, 0, 0)
	cfg, err := ingestQueueConfig()
	if err != nil {
		return noop, nil, fmt.Errorf("queue configuration: %w", err)
	}
	client, err := start(context.Background(), pool, runner, store, cfg)
	if err != nil {
		return noop, nil, fmt.Errorf("queue start: %w", err)
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Stop(ctx); err != nil {
			log.Printf("river stop: %v", err)
		}
	}, runner, nil
}

func ingestQueueConfig() (queue.Config, error) {
	cfg := queue.Config{
		IngestInterval: durEnv("INGEST_INTERVAL", 6*time.Hour),
		SweepInterval:  durEnv("SWEEP_INTERVAL", time.Hour),
		StaleAfter:     durEnv("STALE_AFTER", ingest.StaleAfter),
	}
	if err := cfg.Validate(); err != nil {
		return queue.Config{}, fmt.Errorf("STALE_AFTER: %w", err)
	}
	return cfg, nil
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
	if sourceURL := os.Getenv("E2E_SOURCE_URL"); sourceURL != "" {
		for _, job := range store.All() {
			job.SourceURL = sourceURL
			if _, err := store.Upsert(context.Background(), job); err != nil {
				log.Printf("seed fixture source URL: %v", err)
				return
			}
		}
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

func newMailer() (auth.Mailer, error) {
	addr := os.Getenv("SMTP_ADDR")
	if addr == "" {
		return nil, nil
	}
	return auth.NewSMTPMailer(addr, os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"), os.Getenv("SMTP_FROM"))
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
func newCVStore(ctx context.Context) (storage.ObjectStore, error) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		if os.Getenv("DATABASE_URL") != "" {
			return nil, errors.New("S3_ENDPOINT is required when DATABASE_URL is set")
		}
		key, err := decodeCVKey(false)
		if err != nil {
			return nil, err
		}
		return storage.NewEncryptedStore(storage.NewMemoryStore(), key)
	}
	key, err := decodeCVKey(true)
	if err != nil {
		return nil, err
	}
	backend, err := storage.NewS3Store(ctx, storage.S3Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    os.Getenv("S3_BUCKET"),
		UseTLS:    boolEnv("S3_USE_TLS", true),
	})
	if err != nil {
		return nil, err
	}
	return storage.NewEncryptedStore(backend, key)
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

func decodeCVKey(required bool) ([]byte, error) {
	if hexKey := os.Getenv("CV_ENCRYPTION_KEY"); hexKey != "" {
		key, err := hex.DecodeString(hexKey)
		if err != nil || len(key) != 32 {
			return nil, errors.New("CV_ENCRYPTION_KEY must be 64 hex chars (32 bytes)")
		}
		return key, nil
	}
	if required {
		return nil, errors.New("CV_ENCRYPTION_KEY is required with persistent object storage")
	}
	log.Println("CV_ENCRYPTION_KEY not set; generating an ephemeral dev key")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate cv key: %w", err)
	}
	return key, nil
}
