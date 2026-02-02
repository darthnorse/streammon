package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/store"
)

type Server struct {
	router chi.Router
	store  *store.Store
}

func NewServer(s *store.Store) *Server {
	srv := &Server{
		router: chi.NewRouter(),
		store:  s,
	}
	srv.router.Use(middleware.Logger)
	srv.router.Use(middleware.Recoverer)
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
