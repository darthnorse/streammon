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
	"streammon/internal/crypto"
	"streammon/internal/geoip"
	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/notifier"
	"streammon/internal/poller"
	"streammon/internal/rules"
	"streammon/internal/scheduler"
	"streammon/internal/server"
	"streammon/internal/store"
	"streammon/internal/version"
)

var Version = "dev"

func main() {
	dbPath := envOr("DB_PATH", "./data/streammon.db")
	listenAddr := envOr("LISTEN_ADDR", ":7935")
	migrationsDir := envOr("MIGRATIONS_DIR", "./migrations")
	corsOrigin := os.Getenv("CORS_ORIGIN")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}

	var storeOpts []store.Option
	if encKey := os.Getenv("TOKEN_ENCRYPTION_KEY"); encKey != "" {
		enc, err := crypto.NewEncryptor(encKey)
		if err != nil {
			log.Fatalf("invalid TOKEN_ENCRYPTION_KEY: %v", err)
		}
		storeOpts = append(storeOpts, store.WithEncryptor(enc))
		log.Println("Token encryption enabled")
	} else {
		log.Println("TOKEN_ENCRYPTION_KEY not set — Plex token storage for Overseerr disabled")
	}

	s, err := store.New(dbPath, storeOpts...)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer s.Close()

	if err := s.Migrate(migrationsDir); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	if err := s.CleanupZombieSessions(); err != nil {
		log.Printf("zombie session cleanup: %v", err)
	}

	geoDBPath := envOr("GEOIP_DB", "./geoip/GeoLite2-City.mmdb")
	geoResolver := geoip.NewResolver(geoDBPath)
	defer geoResolver.Close()

	geoUpdater := geoip.NewUpdater(s, geoResolver, geoDBPath)

	// Initialize auth manager with providers
	authMgr := auth.NewManager(s)

	// Register local provider (always available)
	authMgr.RegisterProvider(auth.NewLocalProvider(s, authMgr))

	// Register Plex provider
	plexProvider := auth.NewPlexProvider(s, authMgr)
	authMgr.RegisterProvider(plexProvider)

	// Register Emby and Jellyfin providers
	authMgr.RegisterProvider(auth.NewMediaServerProvider(s, authMgr, models.ServerTypeEmby))
	authMgr.RegisterProvider(auth.NewMediaServerProvider(s, authMgr, models.ServerTypeJellyfin))

	// Register OIDC provider
	oidcCfg, err := s.GetOIDCConfig()
	if err != nil {
		log.Fatalf("loading OIDC config: %v", err)
	}
	oidcProvider, err := auth.NewOIDCProvider(auth.ConfigFromStore(oidcCfg), s, authMgr)
	if err != nil {
		log.Printf("OIDC init failed: %v", err)
	} else {
		authMgr.RegisterProvider(oidcProvider)
		if oidcProvider.Enabled() {
			log.Println("OIDC authentication enabled")
		}
	}

	log.Printf("Auth providers: %v", authMgr.GetEnabledProviders())

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
		poller.WithGeoResolver(geoResolver),
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

	var schOpts []scheduler.Option
	if v := os.Getenv("SCHEDULER_SYNC_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
			schOpts = append(schOpts, scheduler.WithSyncTimeout(d))
		}
	}
	sch := scheduler.New(s, p, schOpts...)

	vc := version.NewChecker(Version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := []server.Option{
		server.WithPoller(p),
		server.WithGeoResolver(geoResolver),
		server.WithAuthManager(authMgr),
		server.WithGeoUpdater(geoUpdater),
		server.WithRulesEngine(rulesEngine),
		server.WithVersion(vc),
		server.WithAppContext(ctx),
	}
	if corsOrigin != "" {
		opts = append(opts, server.WithCORSOrigin(corsOrigin))
	}
	srv := server.NewServer(s, opts...)

	httpServer := &http.Server{
		Addr:              listenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	go vc.Start(ctx)
	go geoUpdater.Start(ctx)
	sch.Start(ctx)
	defer sch.Stop()

	go func() {
		log.Printf("StreamMon %s listening on %s", Version, listenAddr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
	p.PersistActiveSessions()
	p.Stop()
	srv.WaitEnrichment()
	srv.WaitAutoSync()
	rulesEngine.WaitForNotifications()
	server.StopRateLimiter()
	server.StopAuthRateLimiter()
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
