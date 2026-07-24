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
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// NOTE (M1): auth currently uses an in-memory repo. A pgx-backed Repo +
	// startup migration apply lands with the M1 DB-integration sub-task.
	authSvc := auth.NewService(auth.NewMemoryRepo(), auth.Config{})

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
