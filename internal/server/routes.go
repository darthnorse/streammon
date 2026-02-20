package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

func (s *Server) routes() {
	s.router.Get("/api/health", s.handleHealth)
	// Public (no auth) so the frontend can show version before login
	s.router.Get("/api/version", s.handleVersion)

	// Multi-provider auth routes (new)
	if s.authManager != nil {
		// Setup endpoints (only work when no users exist)
		s.router.Route("/api/setup", func(r chi.Router) {
			r.Use(limitBody)
			r.Use(corsMiddleware(s.corsOrigin))
			r.Get("/status", s.authManager.HandleGetStatus)
			r.With(RequireSetup(s.authManager), RateLimitAuth).Post("/local", s.handleSetupLocal)
			r.With(RequireSetup(s.authManager), RateLimitAuth).Post("/plex", s.handleSetupPlex)
			r.With(RequireSetup(s.authManager), RateLimitAuth).Post("/emby", s.handleSetupEmby)
			r.With(RequireSetup(s.authManager), RateLimitAuth).Post("/jellyfin", s.handleSetupJellyfin)
		})

		// Auth endpoints
		s.router.Route("/auth", func(r chi.Router) {
			r.Use(limitBody)
			r.Use(corsMiddleware(s.corsOrigin))
			r.Get("/providers", s.authManager.HandleGetProviders)
			r.Post("/logout", s.authManager.HandleLogout)

			// Login endpoints require setup to be complete (prevents creating users before admin exists)
			r.With(RequireSetupComplete(s.authManager), RateLimitAuth).Post("/local/login", s.handleLocalLogin)
			r.With(RequireSetupComplete(s.authManager), RateLimitAuth).Post("/plex/login", s.handlePlexLogin)
			r.With(RequireSetupComplete(s.authManager)).Get("/emby/servers", s.handleEmbyServers)
			r.With(RequireSetupComplete(s.authManager), RateLimitAuth).Post("/emby/login", s.handleEmbyLogin)
			r.With(RequireSetupComplete(s.authManager)).Get("/jellyfin/servers", s.handleJellyfinServers)
			r.With(RequireSetupComplete(s.authManager), RateLimitAuth).Post("/jellyfin/login", s.handleJellyfinLogin)
			r.With(RequireSetupComplete(s.authManager)).Get("/oidc/login", s.handleOIDCLogin)
			r.With(RequireSetupComplete(s.authManager)).Get("/oidc/callback", s.handleOIDCCallback)
		})
	}

	s.router.Route("/api", func(r chi.Router) {
		r.Use(limitBody)
		r.Use(jsonContentType)
		r.Use(corsMiddleware(s.corsOrigin))

		if s.authManager != nil {
			r.Use(RequireAuthManager(s.authManager))
		}

		r.Get("/me", s.handleMe)
		r.Put("/me", s.handleUpdateProfile)
		r.With(RateLimitAuth).Post("/me/password", s.handleChangePassword)

		r.Get("/servers", s.handleListServers)
		r.With(RequireRole(models.RoleAdmin)).Post("/servers", s.handleCreateServer)
		r.With(RequireRole(models.RoleAdmin)).Post("/servers/test", s.handleTestServerAdHoc)
		r.Get("/servers/{id}", s.handleGetServer)
		r.With(RequireRole(models.RoleAdmin)).Put("/servers/{id}", s.handleUpdateServer)
		r.With(RequireRole(models.RoleAdmin)).Delete("/servers/{id}", s.handleDeleteServer)
		r.With(RequireRole(models.RoleAdmin)).Post("/servers/{id}/restore", s.handleRestoreServer)
		r.With(RequireRole(models.RoleAdmin)).Post("/servers/{id}/test", s.handleTestServer)

		r.Get("/history", s.handleListHistory)
		r.Get("/history/daily", s.handleDailyHistory)
		r.Get("/history/{id}/sessions", s.handleListSessions)

		r.Get("/users", s.handleListUsers)
		r.Get("/users/summary", s.handleListUserSummaries)
		r.With(RequireRole(models.RoleAdmin)).Post("/users/sync-avatars", s.handleSyncUserAvatars)
		r.Get("/users/{name}", s.handleGetUser)
		r.Get("/users/{name}/locations", s.handleGetUserLocations)
		r.Get("/users/{name}/stats", s.handleGetUserStats)

		r.Get("/dashboard/sessions", s.handleDashboardSessions)
		r.Get("/dashboard/recent-media", s.handleGetRecentMedia)

		r.Get("/geoip/{ip}", s.handleGeoIPLookup)

		r.Get("/stats", s.handleGetStats)
		r.With(RequireRole(models.RoleAdmin)).Get("/libraries", s.handleGetLibraries)

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
			td := s.tautulliDeps()
			sr.Get("/", s.handleGetIntegrationSettings(td))
			sr.Put("/", s.handleUpdateIntegrationSettings(td))
			sr.Delete("/", s.handleDeleteIntegrationSettings(td))
			sr.Post("/test", s.handleTestIntegrationConnection(td))
			sr.Post("/import", s.handleTautulliImport)
			sr.Post("/enrich", s.handleStartEnrichment)
			sr.Post("/enrich/stop", s.handleStopEnrichment)
			sr.Get("/enrich/status", s.handleEnrichmentStatus)
		})

		r.Route("/settings/overseerr", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			od := s.overseerrDeps()
			sr.Get("/", s.handleGetIntegrationSettings(od))
			sr.Put("/", s.handleUpdateIntegrationSettings(od))
			sr.Delete("/", s.handleDeleteIntegrationSettings(od))
			sr.Post("/test", s.handleTestIntegrationConnection(od))
		})

		r.Route("/settings/sonarr", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sd := s.sonarrDeps()
			sr.Get("/", s.handleGetIntegrationSettings(sd))
			sr.Put("/", s.handleUpdateIntegrationSettings(sd))
			sr.Delete("/", s.handleDeleteIntegrationSettings(sd))
			sr.Post("/test", s.handleTestIntegrationConnection(sd))
		})

		r.Route("/settings/radarr", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			rd := s.radarrDeps()
			sr.Get("/", s.handleGetIntegrationSettings(rd))
			sr.Put("/", s.handleUpdateIntegrationSettings(rd))
			sr.Delete("/", s.handleDeleteIntegrationSettings(rd))
			sr.Post("/test", s.handleTestIntegrationConnection(rd))
		})

		r.Route("/sonarr", func(sr chi.Router) {
			sr.Get("/configured", s.handleIntegrationConfigured(s.sonarrDeps()))
			sr.Get("/calendar", s.handleSonarrCalendar)
			sr.Get("/series/{id}", s.handleSonarrSeries)
			sr.Post("/series/statuses", s.handleSonarrSeriesStatuses)
		})

		r.Get("/radarr/configured", s.handleIntegrationConfigured(s.radarrDeps()))

		r.Route("/tmdb", func(sr chi.Router) {
			sr.Use(s.tmdbRequired)
			sr.Get("/search", s.handleTMDBSearch)
			sr.Get("/discover/*", s.handleTMDBDiscover)
			sr.Get("/movie/{id}", s.handleTMDBMovie)
			sr.Get("/tv/{id}", s.handleTMDBTV)
			sr.Get("/person/{id}", s.handleTMDBPerson)
			sr.Get("/collection/{id}", s.handleTMDBCollection)
		})

		r.Get("/library/tmdb-ids", s.handleLibraryTMDBIDs)

		r.Route("/overseerr", func(sr chi.Router) {
			sr.Get("/configured", s.handleIntegrationConfigured(s.overseerrDeps()))
			sr.Get("/search", s.handleOverseerrSearch)
			sr.Get("/discover/*", s.handleOverseerrDiscover)
			sr.Get("/movie/{id}", s.handleOverseerrMovie)
			sr.Get("/tv/{id}", s.handleOverseerrTV)
			sr.Get("/tv/{id}/season/{seasonNumber}", s.handleOverseerrTVSeason)
			sr.Get("/requests", s.handleOverseerrListRequests)
			sr.With(RequireRole(models.RoleAdmin)).Get("/requests/count", s.handleOverseerrRequestCount)
			sr.Post("/requests", s.handleOverseerrCreateRequest)
			sr.With(RequireRole(models.RoleAdmin)).Post("/requests/{id}/{action}", s.handleOverseerrRequestAction)
			sr.Delete("/requests/{id}", s.handleOverseerrDeleteRequest)
		})

		r.Route("/settings/display", func(sr chi.Router) {
			sr.Get("/", s.handleGetDisplaySettings)
			sr.With(RequireRole(models.RoleAdmin)).Put("/", s.handleUpdateDisplaySettings)
		})

		r.Route("/settings/idle-timeout", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleGetIdleTimeout)
			sr.Put("/", s.handleUpdateIdleTimeout)
		})

		r.Route("/settings/guest", func(sr chi.Router) {
			sr.Get("/", s.handleGetGuestSettings)
			sr.With(RequireRole(models.RoleAdmin)).Put("/", s.handleUpdateGuestSettings)
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
			mr.Get("/sync/status", s.handleSyncStatus)
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
			mr.Delete("/library-items/{id}", s.handleDeleteLibraryItem)
			mr.Get("/candidates/{id}/cross-server", s.handleCrossServerItems)
			mr.Post("/candidates/bulk-delete", s.handleBulkDeleteCandidates)
		})

		r.Get("/users/{name}/trust", s.handleGetUserTrustScore)
		r.Get("/users/{name}/violations", s.handleGetUserViolations)
		r.Route("/users/{name}/household", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleListHouseholdLocations)
			sr.Post("/", s.handleCreateHouseholdLocation)
			sr.Put("/{id}", s.handleUpdateHouseholdTrusted)
			sr.Delete("/{id}", s.handleDeleteHouseholdLocation)
		})

		r.With(RequireRole(models.RoleAdmin)).Post("/household/calculate", s.handleCalculateHouseholdLocations)

		// Admin user management
		r.Route("/admin/users", func(sr chi.Router) {
			sr.Use(RequireRole(models.RoleAdmin))
			sr.Get("/", s.handleAdminListUsers)
			sr.Post("/merge", s.handleAdminMergeUsers)
			sr.Get("/{id}", s.handleAdminGetUser)
			sr.Put("/{id}/role", s.handleAdminUpdateUserRole)
			sr.Post("/{id}/unlink", s.handleAdminUnlinkUser)
			sr.Delete("/{id}", s.handleAdminDeleteUser)
		})
	})

	s.router.Group(func(r chi.Router) {
		r.Use(corsMiddleware(s.corsOrigin))
		if s.authManager != nil {
			r.Use(RequireAuthManager(s.authManager))
		}
		r.Get("/api/servers/{id}/thumb/*", s.handleThumbProxy)
		r.Get("/api/servers/{id}/items/*", s.handleGetItemDetails)
		r.Get("/api/sonarr/poster/{seriesId}", s.handleSonarrPoster)
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
