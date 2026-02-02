package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) routes() {
	s.router.Get("/api/health", s.handleHealth)

	s.router.Route("/api", func(r chi.Router) {
		r.Use(limitBody)
		r.Use(jsonContentType)
		r.Use(corsMiddleware(s.corsOrigin))

		r.Get("/servers", s.handleListServers)
		r.Post("/servers", s.handleCreateServer)
		r.Post("/servers/test", s.handleTestServerAdHoc)
		r.Get("/servers/{id}", s.handleGetServer)
		r.Put("/servers/{id}", s.handleUpdateServer)
		r.Delete("/servers/{id}", s.handleDeleteServer)
		r.Post("/servers/{id}/test", s.handleTestServer)

		r.Get("/history", s.handleListHistory)
		r.Get("/history/daily", s.handleDailyHistory)

		r.Get("/users", s.handleListUsers)
		r.Get("/users/{name}", s.handleGetUser)
		r.Get("/users/{name}/locations", s.handleGetUserLocations)

		r.Get("/dashboard/sessions", s.handleDashboardSessions)
	})

	s.router.Group(func(r chi.Router) {
		r.Use(corsMiddleware(s.corsOrigin))
		r.Get("/api/dashboard/sse", s.handleDashboardSSE)
	})

	s.serveSPA()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.store.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"error"}`))
		return
	}
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
