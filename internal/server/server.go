package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/store"
)

type Server struct {
	router     chi.Router
	store      *store.Store
	corsOrigin string
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
