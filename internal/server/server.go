package server

import (
	"context"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/auth"
	"streammon/internal/geoip"
	"streammon/internal/httputil"
	"streammon/internal/maintenance"
	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
	"streammon/internal/tmdb"
	"streammon/internal/version"
)

type GeoLookup interface {
	Lookup(ip net.IP) *models.GeoResult
}

type RulesEngine interface {
	InvalidateCache()
}

type Server struct {
	router         chi.Router
	store          *store.Store
	poller         *poller.Poller
	authManager    *auth.Manager
	corsOrigin     string
	geoResolver    GeoLookup
	geoUpdater     *geoip.Updater
	libCache       *libraryCache
	rulesEngine    RulesEngine
	version        *version.Checker
	enrichment     *enrichmentState
	autoSync       autoSyncState
	librarySync    *librarySyncManager
	appCtx         context.Context
	cascadeDeleter   *maintenance.CascadeDeleter
	overseerrUsers   *overseerrUserCache
	tmdbClient       *tmdb.Client
	thumbProxyHTTP   *http.Client
	sonarrPosterHTTP *http.Client
}

func NewServer(s *store.Store, opts ...Option) *Server {
	srv := &Server{
		router:     chi.NewRouter(),
		store:      s,
		libCache:    &libraryCache{},
		enrichment:  &enrichmentState{},
		librarySync: &librarySyncManager{active: make(map[string]*librarySyncJob)},
		appCtx:     context.Background(),
		cascadeDeleter: maintenance.NewCascadeDeleter(s),
		overseerrUsers: &overseerrUserCache{},
		thumbProxyHTTP:   httputil.NewClient(),
		sonarrPosterHTTP: httputil.NewClient(),
	}
	for _, o := range opts {
		o(srv)
	}
	srv.router.Use(middleware.Recoverer)
	srv.router.Use(securityHeaders)
	srv.routes()
	return srv
}

type Option func(*Server)

func WithCORSOrigin(origin string) Option {
	return func(s *Server) { s.corsOrigin = origin }
}

func WithPoller(p *poller.Poller) Option {
	return func(s *Server) { s.poller = p }
}

func WithGeoResolver(r GeoLookup) Option {
	return func(s *Server) { s.geoResolver = r }
}

func WithAuthManager(m *auth.Manager) Option {
	return func(s *Server) { s.authManager = m }
}

func WithGeoUpdater(u *geoip.Updater) Option {
	return func(s *Server) { s.geoUpdater = u }
}

func WithRulesEngine(r RulesEngine) Option {
	return func(s *Server) { s.rulesEngine = r }
}

func WithVersion(v *version.Checker) Option {
	return func(s *Server) { s.version = v }
}

func WithAppContext(ctx context.Context) Option {
	return func(s *Server) { s.appCtx = ctx }
}

func WithTMDBClient(c *tmdb.Client) Option {
	return func(s *Server) { s.tmdbClient = c }
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// WaitEnrichment blocks until any running background enrichment finishes.
func (s *Server) WaitEnrichment() {
	s.enrichment.Wait()
}

// WaitAutoSync blocks until any running server auto-syncs (on add/update) finish.
func (s *Server) WaitAutoSync() {
	s.autoSync.Wait()
}

// WaitLibrarySync blocks until any running on-demand library syncs finish.
func (s *Server) WaitLibrarySync() {
	s.librarySync.Wait()
}
