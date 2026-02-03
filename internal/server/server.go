package server

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/auth"
	"streammon/internal/geoip"
	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
)

type GeoLookup interface {
	Lookup(ip net.IP) *models.GeoResult
}

type Server struct {
	router      chi.Router
	store       *store.Store
	poller      *poller.Poller
	authService *auth.Service
	corsOrigin  string
	geoResolver GeoLookup
	geoUpdater  *geoip.Updater
}

func NewServer(s *store.Store, opts ...Option) *Server {
	srv := &Server{
		router: chi.NewRouter(),
		store:  s,
	}
	for _, o := range opts {
		o(srv)
	}
	srv.router.Use(middleware.Logger)
	srv.router.Use(middleware.Recoverer)
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

func WithAuth(a *auth.Service) Option {
	return func(s *Server) { s.authService = a }
}

func WithGeoUpdater(u *geoip.Updater) Option {
	return func(s *Server) { s.geoUpdater = u }
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
