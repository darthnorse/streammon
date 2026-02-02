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

	"streammon/internal/auth"
	"streammon/internal/geoip"
	"streammon/internal/media"
	"streammon/internal/poller"
	"streammon/internal/server"
	"streammon/internal/store"
)

func main() {
	dbPath := envOr("DB_PATH", "./data/streammon.db")
	listenAddr := envOr("LISTEN_ADDR", ":7935")
	migrationsDir := envOr("MIGRATIONS_DIR", "./migrations")
	corsOrigin := os.Getenv("CORS_ORIGIN")

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

	geoDBPath := os.Getenv("GEOIP_DB")
	geoResolver := geoip.NewResolver(geoDBPath)
	defer geoResolver.Close()

	oidcCfg := auth.Config{
		Issuer:       os.Getenv("OIDC_ISSUER"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
	}
	authSvc, err := auth.NewService(oidcCfg, s)
	if err != nil {
		log.Fatalf("initializing auth: %v", err)
	}
	if authSvc.Enabled() {
		log.Println("OIDC authentication enabled")
	} else {
		log.Println("OIDC not configured — authentication disabled")
	}

	p := poller.New(s, 15*time.Second)

	servers, err := s.ListServers()
	if err != nil {
		log.Fatalf("loading servers: %v", err)
	}
	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}
		ms, err := media.NewMediaServer(srv)
		if err != nil {
			log.Printf("skipping server %s: %v", srv.Name, err)
			continue
		}
		p.AddServer(srv.ID, ms)
	}

	p.Start(context.Background())
	defer p.Stop()

	var opts []server.Option
	if corsOrigin != "" {
		opts = append(opts, server.WithCORSOrigin(corsOrigin))
	}
	opts = append(opts, server.WithPoller(p))
	opts = append(opts, server.WithGeoResolver(geoResolver))
	opts = append(opts, server.WithAuth(authSvc))
	srv := server.NewServer(s, opts...)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
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
