package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"streammon/internal/auth"
	"streammon/internal/geoip"
	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/notifier"
	"streammon/internal/poller"
	"streammon/internal/rules"
	"streammon/internal/scheduler"
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

	geoDBPath := envOr("GEOIP_DB", "./geoip/GeoLite2-City.mmdb")
	geoResolver := geoip.NewResolver(geoDBPath)
	defer geoResolver.Close()

	geoUpdater := geoip.NewUpdater(s, geoResolver, geoDBPath)

	oidcCfg, err := s.GetOIDCConfig()
	if err != nil {
		log.Fatalf("loading OIDC config: %v", err)
	}
	authSvc, err := auth.NewService(auth.ConfigFromStore(oidcCfg), s)
	if err != nil {
		log.Printf("OIDC init failed (starting with auth disabled): %v", err)
		authSvc, _ = auth.NewService(auth.Config{}, s)
	}
	if authSvc.Enabled() {
		log.Println("OIDC authentication enabled")
	} else {
		log.Println("OIDC not configured — authentication disabled")
	}

	// Initialize rules engine
	rulesGeo := &geoAdapter{resolver: geoResolver}
	rulesEngine := rules.NewEngine(s, rulesGeo, rules.DefaultEngineConfig())
	rulesEngine.SetNotifier(notifier.New())

	pollInterval := 5 * time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 2*time.Second {
			pollInterval = d
		}
	}

	// Household auto-learning: enabled by default with 10 sessions threshold
	// Set HOUSEHOLD_AUTOLEARN_MIN_SESSIONS=0 to disable auto-learning
	autoLearnMinSessions := poller.DefaultAutoLearnMinSessions
	if v := os.Getenv("HOUSEHOLD_AUTOLEARN_MIN_SESSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			autoLearnMinSessions = n
		}
	}

	p := poller.New(s, pollInterval,
		poller.WithRulesEngine(rulesEngine),
		poller.WithHouseholdAutoLearn(autoLearnMinSessions),
	)

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

	var schOpts []scheduler.Option
	if v := os.Getenv("SCHEDULER_SYNC_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
			schOpts = append(schOpts, scheduler.WithSyncTimeout(d))
		}
	}
	sch := scheduler.New(s, p, schOpts...)

	opts := []server.Option{
		server.WithPoller(p),
		server.WithGeoResolver(geoResolver),
		server.WithAuth(authSvc),
		server.WithGeoUpdater(geoUpdater),
		server.WithRulesEngine(rulesEngine),
	}
	if corsOrigin != "" {
		opts = append(opts, server.WithCORSOrigin(corsOrigin))
	}
	srv := server.NewServer(s, opts...)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go geoUpdater.Start(ctx)
	sch.Start(ctx)
	defer sch.Stop()

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

// geoAdapter adapts geoip.Resolver to the rules.GeoResolver interface.
type geoAdapter struct {
	resolver *geoip.Resolver
}

func (g *geoAdapter) Lookup(_ context.Context, ip string) (*models.GeoResult, error) {
	if g.resolver == nil {
		return nil, nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, nil
	}
	return g.resolver.Lookup(parsed), nil
}
