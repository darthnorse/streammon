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
		r.Get("/users/summary", s.handleListUserSummaries)
		r.Post("/users/sync-avatars", s.handleSyncUserAvatars)
		r.Get("/users/{name}", s.handleGetUser)
		r.Get("/users/{name}/locations", s.handleGetUserLocations)
		r.Get("/users/{name}/stats", s.handleGetUserStats)

		r.Get("/dashboard/sessions", s.handleDashboardSessions)
		r.Get("/dashboard/recent-media", s.handleGetRecentMedia)

		r.Get("/geoip/{ip}", s.handleGeoIPLookup)

		r.Get("/stats", s.handleGetStats)
		r.Get("/libraries", s.handleGetLibraries)

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
			sr.Post("/backfill", s.handleGeoBackfill)
		})

		r.Route("/settings/tautulli", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleGetTautulliSettings)
			sr.Put("/", s.handleUpdateTautulliSettings)
			sr.Delete("/", s.handleDeleteTautulliSettings)
			sr.Post("/test", s.handleTestTautulliConnection)
			sr.Post("/import", s.handleTautulliImport)
		})

		r.Route("/rules", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleListRules)
			sr.Post("/", s.handleCreateRule)
			sr.Get("/{id}", s.handleGetRule)
			sr.Put("/{id}", s.handleUpdateRule)
			sr.Delete("/{id}", s.handleDeleteRule)
			sr.Post("/{id}/channels", s.handleLinkRuleToChannel)
			sr.Delete("/{id}/channels/{channelId}", s.handleUnlinkRuleFromChannel)
			sr.Get("/{id}/channels", s.handleGetRuleChannels)
		})

		r.Route("/violations", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleListViolations)
		})

		r.Route("/notifications", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleListNotificationChannels)
			sr.Post("/", s.handleCreateNotificationChannel)
			sr.Get("/{id}", s.handleGetNotificationChannel)
			sr.Put("/{id}", s.handleUpdateNotificationChannel)
			sr.Delete("/{id}", s.handleDeleteNotificationChannel)
			sr.Post("/{id}/test", s.handleTestNotificationChannel)
		})

		// Maintenance routes (admin only)
		r.Route("/maintenance", func(mr chi.Router) {
			mr.Use(RequireRole(models.RoleAdmin))
			mr.Use(rateLimit)
			mr.Get("/criterion-types", s.handleGetCriterionTypes)
			mr.Get("/dashboard", s.handleGetMaintenanceDashboard)
			mr.Post("/sync", s.handleSyncLibraryItems)
			mr.Get("/rules", s.handleListMaintenanceRules)
			mr.Post("/rules", s.handleCreateMaintenanceRule)
			mr.Get("/rules/{id}", s.handleGetMaintenanceRule)
			mr.Put("/rules/{id}", s.handleUpdateMaintenanceRule)
			mr.Delete("/rules/{id}", s.handleDeleteMaintenanceRule)
			mr.Post("/rules/{id}/evaluate", s.handleEvaluateRule)
			mr.Get("/rules/{id}/candidates", s.handleListCandidates)
			mr.Get("/rules/{id}/candidates/export", s.handleExportCandidates)
			mr.Get("/rules/{id}/exclusions", s.handleListExclusions)
			mr.Post("/rules/{id}/exclusions", s.handleCreateExclusions)
			mr.Delete("/rules/{id}/exclusions/{itemId}", s.handleDeleteExclusion)
			mr.Post("/rules/{id}/exclusions/bulk-remove", s.handleBulkRemoveExclusions)
			mr.Delete("/candidates/{id}", s.handleDeleteCandidate)
			mr.Post("/candidates/bulk-delete", s.handleBulkDeleteCandidates)
		})

		r.Get("/users/{name}/trust", s.handleGetUserTrustScore)
		r.Route("/users/{name}/household", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleListHouseholdLocations)
			sr.Post("/", s.handleCreateHouseholdLocation)
			sr.Put("/{id}", s.handleUpdateHouseholdTrusted)
			sr.Delete("/{id}", s.handleDeleteHouseholdLocation)
		})

		r.With(RequireRole(models.RoleAdmin)).Post("/household/calculate", s.handleCalculateHouseholdLocations)
	})

	s.router.Group(func(r chi.Router) {
		r.Use(corsMiddleware(s.corsOrigin))
		if s.authService != nil {
			r.Use(RequireAuth(s.authService))
		}
		r.Get("/api/servers/{id}/thumb/*", s.handleThumbProxy)
		r.Get("/api/servers/{id}/items/*", s.handleGetItemDetails)
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
