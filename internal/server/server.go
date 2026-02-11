package server

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/auth"
	"streammon/internal/geoip"
	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
	"streammon/internal/version"
)

type GeoLookup interface {
	Lookup(ip net.IP) *models.GeoResult
}

// RulesEngine allows the server to invalidate the rules cache when rules are modified.
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
	overseerrUsers     *overseerrUserCache
	overseerrPlexCache *overseerrPlexTokenCache
	warnHTTPOnce       sync.Once
}

func NewServer(s *store.Store, opts ...Option) *Server {
	srv := &Server{
		router:         chi.NewRouter(),
		store:          s,
		libCache:       &libraryCache{},
		overseerrUsers: &overseerrUserCache{},
		overseerrPlexCache: &overseerrPlexTokenCache{
			userIDMap:   make(map[int64]int),
			entryExpiry: make(map[int64]time.Time),
		},
	}
	for _, o := range opts {
		o(srv)
	}
	srv.router.Use(middleware.Logger)
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
