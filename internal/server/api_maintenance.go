package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/maintenance"
	"streammon/internal/models"
)

// GET /api/maintenance/criterion-types
func (s *Server) handleGetCriterionTypes(w http.ResponseWriter, r *http.Request) {
	types := maintenance.GetCriterionTypes()
	writeJSON(w, http.StatusOK, map[string]any{"types": types})
}

// GET /api/maintenance/dashboard
func (s *Server) handleGetMaintenanceDashboard(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	var libraries []models.LibraryMaintenance

	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}

		ms, ok := s.poller.GetServer(srv.ID)
		if !ok {
			continue
		}

		libs, err := ms.GetLibraries(r.Context())
		if err != nil {
			log.Printf("maintenance dashboard: get libraries from %s: %v", srv.Name, err)
			continue
		}

		for _, lib := range libs {
			if lib.Type != models.LibraryTypeMovie && lib.Type != models.LibraryTypeShow {
				continue
			}

			rules, err := s.store.ListMaintenanceRulesWithCounts(r.Context(), srv.ID, lib.ID)
			if err != nil {
				log.Printf("maintenance dashboard: get rules for %s/%s: %v", srv.Name, lib.Name, err)
				continue
			}

			lastSync, _ := s.store.GetLastSyncTime(r.Context(), srv.ID, lib.ID)
			itemCount, _ := s.store.CountLibraryItems(r.Context(), srv.ID, lib.ID)

			libraries = append(libraries, models.LibraryMaintenance{
				ServerID:     srv.ID,
				ServerName:   srv.Name,
				LibraryID:    lib.ID,
				LibraryName:  lib.Name,
				LibraryType:  lib.Type,
				TotalItems:   itemCount,
				LastSyncedAt: lastSync,
				Rules:        rules,
			})
		}
	}

	writeJSON(w, http.StatusOK, models.MaintenanceDashboard{Libraries: libraries})
}

// POST /api/maintenance/sync
func (s *Server) handleSyncLibraryItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID  int64  `json:"server_id"`
		LibraryID string `json:"library_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ms, ok := s.poller.GetServer(req.ServerID)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	syncStart := time.Now().UTC()

	items, err := ms.GetLibraryItems(ctx, req.LibraryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch library items: "+err.Error())
		return
	}

	count, err := s.store.UpsertLibraryItems(ctx, items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save library items: "+err.Error())
		return
	}

	// Delete items not seen in this sync
	deleted, err := s.store.DeleteStaleLibraryItems(ctx, req.ServerID, req.LibraryID, syncStart)
	if err != nil {
		log.Printf("failed to delete stale items: %v", err)
	}

	// Re-evaluate all enabled rules for this library
	rules, err := s.store.ListMaintenanceRules(ctx, req.ServerID, req.LibraryID)
	if err == nil {
		evaluator := maintenance.NewEvaluator(s.store)
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			candidates, evalErr := evaluator.EvaluateRule(ctx, &rule)
			if evalErr != nil {
				log.Printf("evaluate rule %d: %v", rule.ID, evalErr)
				continue
			}

			batch := make([]struct {
				LibraryItemID int64
				Reason        string
			}, len(candidates))
			for i, c := range candidates {
				batch[i] = struct {
					LibraryItemID int64
					Reason        string
				}{c.LibraryItemID, c.Reason}
			}
			if err := s.store.BatchUpsertCandidates(ctx, rule.ID, batch); err != nil {
				log.Printf("upsert candidates for rule %d: %v", rule.ID, err)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"synced":  count,
		"deleted": deleted,
	})
}

// GET /api/maintenance/rules
func (s *Server) handleListMaintenanceRules(w http.ResponseWriter, r *http.Request) {
	serverID, _ := strconv.ParseInt(r.URL.Query().Get("server_id"), 10, 64)
	libraryID := r.URL.Query().Get("library_id")

	rules, err := s.store.ListMaintenanceRulesWithCounts(r.Context(), serverID, libraryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// POST /api/maintenance/rules
func (s *Server) handleCreateMaintenanceRule(w http.ResponseWriter, r *http.Request) {
	var input models.MaintenanceRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := s.store.CreateMaintenanceRule(r.Context(), &input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// GET /api/maintenance/rules/{id}
func (s *Server) handleGetMaintenanceRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	rule, err := s.store.GetMaintenanceRule(r.Context(), id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get rule")
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// PUT /api/maintenance/rules/{id}
func (s *Server) handleUpdateMaintenanceRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var input models.MaintenanceRuleUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := s.store.UpdateMaintenanceRule(r.Context(), id, &input)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// DELETE /api/maintenance/rules/{id}
func (s *Server) handleDeleteMaintenanceRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	if err := s.store.DeleteMaintenanceRule(r.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/maintenance/rules/{id}/evaluate
func (s *Server) handleEvaluateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	rule, err := s.store.GetMaintenanceRule(r.Context(), id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get rule")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	evaluator := maintenance.NewEvaluator(s.store)
	candidates, err := evaluator.EvaluateRule(ctx, rule)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to evaluate rule: "+err.Error())
		return
	}

	batch := make([]struct {
		LibraryItemID int64
		Reason        string
	}, len(candidates))
	for i, c := range candidates {
		batch[i] = struct {
			LibraryItemID int64
			Reason        string
		}{c.LibraryItemID, c.Reason}
	}

	if err := s.store.BatchUpsertCandidates(ctx, rule.ID, batch); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save candidates")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"candidates": len(candidates)})
}

// GET /api/maintenance/rules/{id}/candidates
func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	ruleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || ruleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	result, err := s.store.ListCandidatesForRule(r.Context(), ruleID, page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list candidates")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
