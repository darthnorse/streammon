package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"streammon/internal/server"
	"streammon/internal/store"
)

func main() {
	dbPath := envOr("DB_PATH", "./data/streammon.db")
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	migrationsDir := envOr("MIGRATIONS_DIR", "./migrations")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer s.Close()

	if err := s.Migrate(migrationsDir); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	srv := server.NewServer(s)

	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: srv,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("StreamMon listening on %s", listenAddr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
