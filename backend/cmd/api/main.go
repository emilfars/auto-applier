// Command api starts the Auto Applier backend HTTP server.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auto-applier/backend/internal/api"
	"github.com/auto-applier/backend/internal/auth"
	"github.com/auto-applier/backend/internal/db"
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

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(authSvc.Routes()),
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
