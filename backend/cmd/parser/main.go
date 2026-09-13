// Command parser runs the Auto Applier CV parser service (M5.5). It exposes the
// envelope defined in backend/internal/cv/hosted.go and drives an OpenRouter
// model. It is a separate deployment from the API and is the only process that
// holds OPENROUTER_API_KEY.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/auto-applier/backend/internal/parser"
)

func main() {
	addr := envOr("PARSER_ADDR", ":8090")

	models := splitList(os.Getenv("OPENROUTER_MODELS"))
	if len(models) == 0 {
		models = []string{envOr("OPENROUTER_MODEL", "google/gemini-flash-1.5-8b")}
	}

	engine, err := parser.NewOpenRouter(parser.OpenRouterConfig{
		BaseURL: os.Getenv("OPENROUTER_BASE_URL"),
		APIKey:  os.Getenv("OPENROUTER_API_KEY"),
		Models:  models,
		Referer: os.Getenv("OPENROUTER_REFERER"),
		Title:   os.Getenv("OPENROUTER_TITLE"),
	}, nil)
	if err != nil {
		log.Fatalf("configure OpenRouter engine: %v", err)
	}

	svc := parser.NewService(parser.DefaultExtractor{}, engine, intEnv("PARSER_MAX_BYTES", 10<<20))
	handler := parser.NewServer(svc, os.Getenv("CV_PARSER_API_KEY"))

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("parser listening on %s (model %s)", addr, strings.Join(models, ","))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("parser server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("parser shutdown error: %v", err)
	}
	log.Println("parser stopped")
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func intEnv(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("%s=%q is not an integer; using %d", name, v, def)
	}
	return def
}
