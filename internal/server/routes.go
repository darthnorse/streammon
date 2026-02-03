package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

func (s *Server) routes() {
	s.router.Get("/api/health", s.handleHealth)

	if s.authService != nil {
		s.router.Get("/auth/login", s.authService.HandleLogin)
		s.router.Get("/auth/callback", s.authService.HandleCallback)
		s.router.Post("/auth/logout", s.authService.HandleLogout)
	}

	s.router.Route("/api", func(r chi.Router) {
		r.Use(limitBody)
		r.Use(jsonContentType)
		r.Use(corsMiddleware(s.corsOrigin))
		if s.authService != nil {
			r.Use(RequireAuth(s.authService))
		}

		r.Get("/me", s.handleMe)

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
		r.Get("/dashboard/recent-media", s.handleGetRecentMedia)

		r.Get("/geoip/{ip}", s.handleGeoIPLookup)

		r.Route("/settings/oidc", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleGetOIDCSettings)
			sr.Put("/", s.handleUpdateOIDCSettings)
			sr.Delete("/", s.handleDeleteOIDCSettings)
			sr.Post("/test", s.handleTestOIDCConnection)
		})

		r.Route("/settings/maxmind", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleGetMaxMindSettings)
			sr.Put("/", s.handleUpdateMaxMindSettings)
			sr.Delete("/", s.handleDeleteMaxMindSettings)
		})
	})

	s.router.Group(func(r chi.Router) {
		r.Use(corsMiddleware(s.corsOrigin))
		if s.authService != nil {
			r.Use(RequireAuth(s.authService))
		}
		r.Get("/api/servers/{id}/thumb/*", s.handleThumbProxy)
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
