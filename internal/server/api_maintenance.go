package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/maintenance"
	"streammon/internal/models"
)

// parsePagination extracts and validates page and perPage from query params
func parsePagination(r *http.Request, defaultPerPage, maxPerPage int) (page, perPage int) {
	const maxPage = 10000
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	} else if page > maxPage {
		page = maxPage
	}
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > maxPerPage {
		perPage = defaultPerPage
	}
	return page, perPage
}

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

			lastSync, err := s.store.GetLastSyncTime(r.Context(), srv.ID, lib.ID)
			if err != nil {
				log.Printf("maintenance dashboard: get last sync time for %s/%s: %v", srv.Name, lib.Name, err)
			}
			itemCount, err := s.store.CountLibraryItems(r.Context(), srv.ID, lib.ID)
			if err != nil {
				log.Printf("maintenance dashboard: count items for %s/%s: %v", srv.Name, lib.Name, err)
			}

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

	if req.ServerID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}
	if req.LibraryID == "" {
		writeError(w, http.StatusBadRequest, "library_id is required")
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
		log.Printf("sync library: fetch items failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch library items")
		return
	}

	count, err := s.store.UpsertLibraryItems(ctx, items)
	if err != nil {
		log.Printf("sync library: save items failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save library items")
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
			if err := s.store.BatchUpsertCandidates(ctx, rule.ID, maintenance.ToBatch(candidates)); err != nil {
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
		log.Printf("evaluate rule %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to evaluate rule")
		return
	}

	if err := s.store.BatchUpsertCandidates(ctx, rule.ID, maintenance.ToBatch(candidates)); err != nil {
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
	page, perPage := parsePagination(r, 20, 100)

	result, err := s.store.ListCandidatesForRule(r.Context(), ruleID, page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list candidates")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DELETE /api/maintenance/candidates/{id}
func (s *Server) handleDeleteCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	// Get candidate with item details
	candidate, err := s.store.GetMaintenanceCandidate(r.Context(), id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "candidate not found")
		return
	}
	if err != nil {
		log.Printf("get candidate %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to get candidate")
		return
	}

	// Get media server
	ms, ok := s.poller.GetServer(candidate.Item.ServerID)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found or not configured")
		return
	}

	// Get user for audit
	user := UserFromContext(r.Context())
	deletedBy := "unknown"
	if user != nil {
		deletedBy = user.Email
	}

	// Delete from media server with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var serverDeleted bool
	var errMsg string
	if err := ms.DeleteItem(ctx, candidate.Item.ItemID); err != nil {
		log.Printf("delete item %s from server: %v", candidate.Item.ItemID, err)
		errMsg = err.Error()
	} else {
		serverDeleted = true
	}

	// Record audit log
	if err := s.store.RecordDeleteAction(r.Context(),
		candidate.Item.ServerID,
		candidate.Item.ItemID,
		candidate.Item.Title,
		string(candidate.Item.MediaType),
		candidate.Item.FileSize,
		deletedBy,
		serverDeleted,
		errMsg,
	); err != nil {
		log.Printf("record delete action: %v", err)
	}

	// Only remove from DB if server deletion succeeded
	if !serverDeleted {
		writeError(w, http.StatusInternalServerError, "failed to delete from media server")
		return
	}

	// Delete candidate from DB
	if err := s.store.DeleteMaintenanceCandidate(r.Context(), id); err != nil {
		log.Printf("delete candidate %d: %v", id, err)
	}

	// Delete library item from cache
	if err := s.store.DeleteLibraryItem(r.Context(), candidate.LibraryItemID); err != nil {
		log.Printf("delete library item %d: %v", candidate.LibraryItemID, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/maintenance/rules/{id}/candidates/export?format=csv|json
func (s *Server) handleExportCandidates(w http.ResponseWriter, r *http.Request) {
	ruleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || ruleID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		writeError(w, http.StatusBadRequest, "format must be csv or json")
		return
	}

	candidates, err := s.store.ListAllCandidatesForRule(r.Context(), ruleID)
	if err != nil {
		log.Printf("export candidates: list failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch candidates")
		return
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("candidates-%d-%s.%s", ruleID, timestamp, format)

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		s.exportCandidatesCSV(w, candidates)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		s.exportCandidatesJSON(w, candidates, ruleID)
	}
}

func (s *Server) exportCandidatesCSV(w http.ResponseWriter, candidates []models.MaintenanceCandidate) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header
	cw.Write([]string{"ID", "Title", "Media Type", "Year", "Added At", "Resolution", "File Size (GB)", "Reason", "Computed At"})

	for _, c := range candidates {
		if c.Item == nil {
			continue
		}
		sizeGB := float64(c.Item.FileSize) / (1024 * 1024 * 1024)
		cw.Write([]string{
			strconv.FormatInt(c.ID, 10),
			c.Item.Title,
			string(c.Item.MediaType),
			strconv.Itoa(c.Item.Year),
			c.Item.AddedAt.Format(time.RFC3339),
			c.Item.VideoResolution,
			fmt.Sprintf("%.2f", sizeGB),
			c.Reason,
			c.ComputedAt.Format(time.RFC3339),
		})
	}
}

func (s *Server) exportCandidatesJSON(w http.ResponseWriter, candidates []models.MaintenanceCandidate, ruleID int64) {
	response := map[string]any{
		"rule_id":     ruleID,
		"candidates":  candidates,
		"total":       len(candidates),
		"exported_at": time.Now().UTC(),
	}
	json.NewEncoder(w).Encode(response)
}
