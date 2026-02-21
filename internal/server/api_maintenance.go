package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/maintenance"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

const maxBulkOperationSize = 500  // SQLite SQLITE_MAX_VARIABLE_NUMBER limit is 999
const maxExportSize = 10000       // Prevent OOM on large exports
const maxSearchLength = 200       // Prevent abuse with extremely long search strings

type deleteItemResult struct {
	ServerDeleted bool
	DBCleaned     bool
	FileSize      int64
	Error         string
}

// checkExclusion performs a final exclusion safety check as close to the
// irreversible media server delete as possible, minimising the TOCTOU window.
func (s *Server) checkExclusion(candidate models.MaintenanceCandidate) (excluded bool, err error) {
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer checkCancel()

	if candidate.RuleID > 0 {
		return s.store.IsItemExcluded(checkCtx, candidate.RuleID, candidate.LibraryItemID)
	}
	return s.store.IsItemExcludedFromAnyRule(checkCtx, candidate.LibraryItemID)
}

// Uses background contexts to ensure operations complete even if request is cancelled.
func (s *Server) deleteItemFromServer(candidate models.MaintenanceCandidate, deletedBy string) deleteItemResult {
	result := deleteItemResult{FileSize: candidate.Item.FileSize}

	ms, ok := s.poller.GetServer(candidate.Item.ServerID)
	if !ok {
		result.Error = "server not configured"
		return result
	}

	excluded, err := s.checkExclusion(candidate)
	if err != nil {
		result.Error = "failed to verify exclusion status"
		return result
	}
	if excluded {
		if candidate.RuleID > 0 {
			result.Error = "item was excluded since operation began"
		} else {
			result.Error = "item is excluded from a maintenance rule"
		}
		return result
	}

	deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	deleteErr := ms.DeleteItem(deleteCtx, candidate.Item.ItemID)
	cancel()

	if deleteErr != nil {
		result.Error = deleteErr.Error()
		s.recordDeleteAudit(candidate, deletedBy, false, result.Error)
		return result
	}

	result.ServerDeleted = true

	cascadeResults := s.cascadeDeleter.DeleteExternalReferences(context.Background(), candidate.Item)
	for _, cr := range cascadeResults {
		if cr.Error != "" {
			log.Printf("cascade %s warning for %q: %s", cr.Service, candidate.Item.Title, cr.Error)
		}
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	dbErr := s.store.DeleteLibraryItem(cleanupCtx, candidate.LibraryItemID)
	cleanupCancel()

	if dbErr != nil {
		log.Printf("delete library item %d: %v", candidate.LibraryItemID, dbErr)
		result.Error = "file deleted but database cleanup failed - please refresh"
	} else {
		result.DBCleaned = true
	}

	s.recordDeleteAudit(candidate, deletedBy, result.ServerDeleted, result.Error)

	return result
}

func (s *Server) deleteOldSeasons(candidate models.MaintenanceCandidate, rule *models.MaintenanceRule, deletedBy string) deleteItemResult {
	result := deleteItemResult{} // individual season sizes unknown

	var params models.KeepLatestSeasonsParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		result.Error = "invalid rule parameters"
		return result
	}
	if params.KeepSeasons <= 0 {
		params.KeepSeasons = maintenance.DefaultKeepSeasons
	}

	ms, ok := s.poller.GetServer(candidate.Item.ServerID)
	if !ok {
		result.Error = "server not configured"
		return result
	}

	excluded, err := s.checkExclusion(candidate)
	if err != nil {
		result.Error = "failed to verify exclusion status"
		return result
	}
	if excluded {
		result.Error = "item was excluded since operation began"
		return result
	}

	seasonsCtx, seasonsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	seasons, err := ms.GetSeasons(seasonsCtx, candidate.Item.ItemID)
	seasonsCancel()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get seasons: %v", err)
		return result
	}

	var regular []models.Season
	for _, sea := range seasons {
		if sea.Number > 0 {
			regular = append(regular, sea)
		}
	}
	sort.Slice(regular, func(i, j int) bool { return regular[i].Number < regular[j].Number })

	if len(regular) <= params.KeepSeasons {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.store.DeleteMaintenanceCandidate(cleanCtx, candidate.ID)
		cleanCancel()
		result.ServerDeleted = true
		result.DBCleaned = true
		return result
	}

	toDelete := regular[:len(regular)-params.KeepSeasons]
	deletedCount := 0
	for i, season := range toDelete {
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := ms.DeleteItem(deleteCtx, season.ID)
		deleteCancel()

		if err != nil {
			log.Printf("delete season %d (%q) of %q: %v", season.Number, season.Title, candidate.Item.Title, err)
			result.Error = fmt.Sprintf("failed to delete season %d: %v", season.Number, err)
		} else {
			deletedCount++
			s.recordDeleteAudit(models.MaintenanceCandidate{
				Item: &models.LibraryItemCache{
					ServerID:  candidate.Item.ServerID,
					ItemID:    season.ID,
					Title:     fmt.Sprintf("%s - %s", candidate.Item.Title, season.Title),
					MediaType: models.MediaTypeTV,
					FileSize:  0,
				},
				LibraryItemID: candidate.LibraryItemID,
			}, deletedBy, true, "")
		}
	}

	if deletedCount == 0 {
		if result.Error == "" {
			result.Error = "no seasons were deleted"
		}
		return result
	}

	result.ServerDeleted = true

	sonarrResult := s.cascadeDeleter.UpdateSonarrMonitoring(context.Background(), candidate.Item, params.KeepSeasons)
	if sonarrResult.Error != "" {
		log.Printf("sonarr monitoring update for %q: %s", candidate.Item.Title, sonarrResult.Error)
	}

	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
	dbErr := s.store.DeleteMaintenanceCandidate(cleanCtx, candidate.ID)
	cleanCancel()
	if dbErr != nil {
		log.Printf("delete candidate %d after season cleanup: %v", candidate.ID, dbErr)
		result.Error = "seasons deleted but candidate cleanup failed - please refresh"
	} else {
		result.DBCleaned = true
	}

	return result
}

func (s *Server) recordDeleteAudit(candidate models.MaintenanceCandidate, deletedBy string, success bool, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.store.RecordDeleteAction(ctx,
		candidate.Item.ServerID,
		candidate.Item.ItemID,
		candidate.Item.Title,
		string(candidate.Item.MediaType),
		candidate.Item.FileSize,
		deletedBy,
		success,
		errMsg,
	); err != nil {
		log.Printf("record delete action: %v", err)
	}
}

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

func parseIDParam(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id, err == nil && id > 0
}

func getUserEmail(r *http.Request) string {
	if user := UserFromContext(r.Context()); user != nil {
		return user.Email
	}
	return "unknown"
}

func validateBulkIDs(ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("ids required")
	}
	if len(ids) > maxBulkOperationSize {
		return fmt.Errorf("cannot process more than %d items at once", maxBulkOperationSize)
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid id: must be positive")
		}
		if seen[id] {
			return fmt.Errorf("duplicate id in request")
		}
		seen[id] = true
	}
	return nil
}

// GET /api/maintenance/criterion-types
func (s *Server) handleGetCriterionTypes(w http.ResponseWriter, r *http.Request) {
	types := maintenance.GetCriterionTypes()
	writeJSON(w, http.StatusOK, map[string]any{"types": types})
}

// GET /api/maintenance/dashboard
func (s *Server) handleGetMaintenanceDashboard(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeError(w, http.StatusInternalServerError, "poller not configured")
		return
	}

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
	if s.poller == nil {
		writeError(w, http.StatusInternalServerError, "poller not configured")
		return
	}

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

	key := fmt.Sprintf("%d-%s", req.ServerID, req.LibraryID)
	if !s.librarySync.tryStart(key, req.LibraryID) {
		writeError(w, http.StatusConflict, "sync already in progress")
		return
	}

	s.startBackgroundSync(key, req.ServerID, req.LibraryID)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "started"})
}

func (s *Server) startBackgroundSync(key string, serverID int64, libraryID string) {
	go func() {
		var count int
		var deleted int64
		var err error

		ctx, cancel := context.WithTimeout(s.appCtx, 6*time.Hour)
		defer cancel()

		progressCtx, progressCh := mediautil.ContextWithProgress(ctx)

		defer func() {
			if r := recover(); r != nil {
				log.Printf("background sync %s: panic: %v", key, r)
				mediautil.CloseProgress(progressCtx)
				s.librarySync.finish(key, 0, 0, fmt.Errorf("internal error"))
				return
			}
			s.librarySync.finish(key, count, int(deleted), err)
		}()

		log.Printf("background sync %s: started", key)

		done := make(chan struct{})
		go func() {
			defer close(done)
			var lastLog int
			for p := range progressCh {
				s.librarySync.updateProgress(key, p)
				if p.Phase == mediautil.PhaseHistory && p.Current-lastLog >= 1000 {
					log.Printf("background sync %s: history %d/%d", key, p.Current, p.Total)
					lastLog = p.Current
				}
			}
		}()

		var candidates int
		count, deleted, candidates, err = s.executeSyncLibrary(progressCtx, serverID, libraryID)
		mediautil.CloseProgress(progressCtx)
		<-done

		if err != nil {
			var se *syncError
			if errors.As(err, &se) {
				if se.logMessage != "" {
					log.Printf("background sync %s: %s", key, se.logMessage)
				}
			} else {
				log.Printf("background sync %s: %v", key, err)
			}
		} else {
			log.Printf("background sync %s: completed (synced=%d deleted=%d candidates=%d)", key, count, deleted, candidates)
		}
	}()
}

// GET /api/maintenance/sync/status
func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.librarySync.status())
}

type syncError struct {
	message    string
	logMessage string
}

func (e *syncError) Error() string {
	if e.logMessage != "" {
		return e.logMessage
	}
	return e.message
}

func (s *Server) executeSyncLibrary(ctx context.Context, serverID int64, libraryID string) (int, int64, int, error) {
	originalServer, err := s.store.GetServer(serverID)
	if err != nil {
		return 0, 0, 0, &syncError{message: "server not found"}
	}

	ms, ok := s.poller.GetServer(serverID)
	if !ok {
		return 0, 0, 0, &syncError{message: "server not found in poller"}
	}

	items, err := ms.GetLibraryItems(ctx, libraryID)
	if err != nil {
		return 0, 0, 0, &syncError{message: "failed to fetch library items", logMessage: fmt.Sprintf("fetch items failed: %v", err)}
	}

	currentServer, err := s.store.GetServer(serverID)
	if err != nil {
		return 0, 0, 0, &syncError{message: "server not found"}
	}
	if currentServer.URL != originalServer.URL ||
		currentServer.Type != originalServer.Type ||
		currentServer.MachineID != originalServer.MachineID {
		return 0, 0, 0, &syncError{
			message:    "server configuration changed during sync, please retry",
			logMessage: fmt.Sprintf("server %d identity changed during sync, aborting", serverID),
		}
	}

	count, deleted, err := s.store.SyncLibraryItems(ctx, serverID, libraryID, items)
	if err != nil {
		return 0, 0, 0, &syncError{message: "failed to save library items", logMessage: fmt.Sprintf("save items failed: %v", err)}
	}

	var candidateCount int
	rules, err := s.store.ListMaintenanceRules(ctx, serverID, libraryID)
	if err != nil {
		log.Printf("list maintenance rules for %d/%s: %v", serverID, libraryID, err)
	} else {
		evaluator := maintenance.NewEvaluator(s.store, s.tmdbClient, s.poller)
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			candidates, evalErr := evaluator.EvaluateRule(ctx, &rule)
			if evalErr != nil {
				log.Printf("evaluate rule %d: %v", rule.ID, evalErr)
				continue
			}
			candidateCount += len(candidates)
			if err := s.store.BatchUpsertCandidates(ctx, rule.ID, candidates); err != nil {
				log.Printf("upsert candidates for rule %d: %v", rule.ID, err)
			}
		}
	}

	s.InvalidateLibraryCache()
	return count, deleted, candidateCount, nil
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
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	id, ok := parseIDParam(r, "id")
	if !ok {
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
	id, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var input models.MaintenanceRuleUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	id, ok := parseIDParam(r, "id")
	if !ok {
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
	id, ok := parseIDParam(r, "id")
	if !ok {
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

	evaluator := maintenance.NewEvaluator(s.store, s.tmdbClient, s.poller)
	candidates, err := evaluator.EvaluateRule(ctx, rule)
	if err != nil {
		log.Printf("evaluate rule %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to evaluate rule")
		return
	}

	if err := s.store.BatchUpsertCandidates(ctx, rule.ID, candidates); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save candidates")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"candidates": len(candidates)})
}

// GET /api/maintenance/rules/{id}/candidates
func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}
	page, perPage := parsePagination(r, 20, 100)
	search := r.URL.Query().Get("search")
	if len(search) > maxSearchLength {
		writeError(w, http.StatusBadRequest, "search term too long")
		return
	}
	sortBy := r.URL.Query().Get("sort_by")
	switch sortBy {
	case "title", "year", "resolution", "size", "reason", "added_at", "watches":
	default:
		sortBy = ""
	}
	sortOrder := r.URL.Query().Get("sort_order")
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = ""
	}

	var filterServerID int64
	if sid := r.URL.Query().Get("server_id"); sid != "" {
		if v, err := strconv.ParseInt(sid, 10, 64); err == nil {
			filterServerID = v
		}
	}
	filterLibraryID := r.URL.Query().Get("library_id")

	result, err := s.store.ListCandidatesForRule(r.Context(), ruleID, page, perPage, search, sortBy, sortOrder, filterServerID, filterLibraryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list candidates")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DELETE /api/maintenance/candidates/{id}
func (s *Server) handleDeleteCandidate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	if s.poller == nil {
		writeError(w, http.StatusInternalServerError, "poller not configured")
		return
	}

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

	rule, err := s.store.GetMaintenanceRule(r.Context(), candidate.RuleID)
	if err != nil {
		log.Printf("get rule for candidate %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to get rule")
		return
	}

	var result deleteItemResult
	if rule.CriterionType == models.CriterionKeepLatestSeasons {
		result = s.deleteOldSeasons(*candidate, rule, getUserEmail(r))
	} else {
		result = s.deleteItemFromServer(*candidate, getUserEmail(r))
	}

	if !result.ServerDeleted {
		log.Printf("delete candidate %d (%q): %s", id, candidate.Item.Title, result.Error)
		status := http.StatusInternalServerError
		msg := "failed to delete from media server"
		if result.Error != "" {
			msg = result.Error
		}
		writeError(w, status, msg)
		return
	}
	if !result.DBCleaned {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/maintenance/library-items/{id}?source_item_id=N — delete a cross-server library item.
// Requires source_item_id to verify the target is a genuine cross-server match of the source.
func (s *Server) handleDeleteLibraryItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	sourceIDStr := r.URL.Query().Get("source_item_id")
	if sourceIDStr == "" {
		writeError(w, http.StatusBadRequest, "source_item_id is required")
		return
	}
	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil || sourceID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid source_item_id")
		return
	}

	if s.poller == nil {
		writeError(w, http.StatusInternalServerError, "poller not configured")
		return
	}

	sourceItem, err := s.store.GetLibraryItem(r.Context(), sourceID)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "source item not found")
		return
	}
	if err != nil {
		log.Printf("get source item %d: %v", sourceID, err)
		writeError(w, http.StatusInternalServerError, "failed to get source item")
		return
	}

	matches, err := s.store.FindMatchingItems(r.Context(), sourceItem)
	if err != nil {
		log.Printf("find matching items for source %d: %v", sourceID, err)
		writeError(w, http.StatusInternalServerError, "failed to verify cross-server match")
		return
	}

	matched := false
	for _, m := range matches {
		if m.ID == id {
			matched = true
			break
		}
	}
	if !matched {
		writeError(w, http.StatusForbidden, "target item is not a cross-server match of the source")
		return
	}

	// Check if the target item is excluded from any maintenance rule.
	// Exclusions are per-rule, but a cross-server delete has no rule context —
	// honour any exclusion as a signal the user wants to protect this item.
	excluded, err := s.store.IsItemExcludedFromAnyRule(r.Context(), id)
	if err != nil {
		log.Printf("check any-rule exclusion for item %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to verify exclusion status")
		return
	}
	if excluded {
		writeError(w, http.StatusConflict, "item is excluded from a maintenance rule and cannot be deleted")
		return
	}

	item, err := s.store.GetLibraryItem(r.Context(), id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "library item not found")
		return
	}
	if err != nil {
		log.Printf("get library item %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to get library item")
		return
	}

	synthetic := models.MaintenanceCandidate{
		LibraryItemID: item.ID,
		Item:          item,
	}
	result := s.deleteItemFromServer(synthetic, getUserEmail(r))

	if !result.ServerDeleted {
		log.Printf("delete library item %d (%q): %s", id, item.Title, result.Error)
		writeError(w, http.StatusInternalServerError, "failed to delete from media server")
		return
	}
	if !result.DBCleaned {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/maintenance/rules/{id}/candidates/export?format=csv|json
func (s *Server) handleExportCandidates(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
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

	if len(candidates) > maxExportSize {
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("too many candidates to export (%d). Maximum is %d. Please filter or paginate.", len(candidates), maxExportSize))
		return
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("candidates-%d-%s.%s", ruleID, timestamp, format)

	// Buffer output in memory first to detect errors before sending headers
	if format == "csv" {
		data, err := exportCandidatesCSV(candidates)
		if err != nil {
			log.Printf("csv export error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to generate CSV export")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Write(data)
	} else {
		data, err := exportCandidatesJSON(candidates, ruleID)
		if err != nil {
			log.Printf("json export error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to generate JSON export")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Write(data)
	}
}

func exportCandidatesCSV(candidates []models.MaintenanceCandidate) ([]byte, error) {
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
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

	cw.Flush()
	if err := cw.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func exportCandidatesJSON(candidates []models.MaintenanceCandidate, ruleID int64) ([]byte, error) {
	response := map[string]any{
		"rule_id":     ruleID,
		"candidates":  candidates,
		"total":       len(candidates),
		"exported_at": time.Now().UTC(),
	}
	return json.Marshal(response)
}

// GET /api/maintenance/rules/{id}/exclusions
func (s *Server) handleListExclusions(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}
	page, perPage := parsePagination(r, 20, 100)
	search := r.URL.Query().Get("search")
	if len(search) > maxSearchLength {
		writeError(w, http.StatusBadRequest, "search term too long")
		return
	}

	result, err := s.store.ListExclusionsForRule(r.Context(), ruleID, page, perPage, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list exclusions")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// POST /api/maintenance/rules/{id}/exclusions
func (s *Server) handleCreateExclusions(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var req struct {
		LibraryItemIDs []int64 `json:"library_item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateBulkIDs(req.LibraryItemIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	excludedBy := getUserEmail(r)

	count, err := s.store.CreateExclusions(r.Context(), ruleID, req.LibraryItemIDs, excludedBy)
	if err != nil {
		log.Printf("create exclusions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create exclusions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"excluded": count})
}

// DELETE /api/maintenance/rules/{id}/exclusions/{itemId}
func (s *Server) handleDeleteExclusion(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	itemID, itemOK := parseIDParam(r, "itemId")
	if !itemOK {
		writeError(w, http.StatusBadRequest, "invalid itemId parameter")
		return
	}

	if err := s.store.DeleteExclusion(r.Context(), ruleID, itemID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "exclusion not found")
			return
		}
		log.Printf("delete exclusion: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete exclusion")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/maintenance/rules/{id}/exclusions/bulk-remove
func (s *Server) handleBulkRemoveExclusions(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	var req struct {
		LibraryItemIDs []int64 `json:"library_item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateBulkIDs(req.LibraryItemIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	count, err := s.store.DeleteExclusions(r.Context(), ruleID, req.LibraryItemIDs)
	if err != nil {
		log.Printf("delete exclusions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to remove exclusions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"removed": count})
}

// POST /api/maintenance/candidates/bulk-delete
// Streams progress via SSE if Accept: text/event-stream, otherwise returns JSON
func (s *Server) handleBulkDeleteCandidates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CandidateIDs       []int64 `json:"candidate_ids"`
		IncludeCrossServer bool    `json:"include_cross_server"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateBulkIDs(req.CandidateIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.poller == nil {
		writeError(w, http.StatusInternalServerError, "poller not configured")
		return
	}

	// Get all candidates - use background context to ensure we get the data
	// even if request context is starting to be cancelled
	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
	candidates, err := s.store.GetMaintenanceCandidates(fetchCtx, req.CandidateIDs)
	fetchCancel()
	if err != nil {
		log.Printf("get candidates for bulk delete: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get candidates")
		return
	}

	candidateMap := make(map[int64]models.MaintenanceCandidate)
	for _, c := range candidates {
		candidateMap[c.ID] = c
	}

	if len(candidates) != len(req.CandidateIDs) {
		log.Printf("bulk delete: requested %d candidates but only found %d (some may have been removed by sync)",
			len(req.CandidateIDs), len(candidates))
	}

	deletedBy := getUserEmail(r)

	if r.Header.Get("Accept") == "text/event-stream" {
		s.streamBulkDelete(w, r, req.CandidateIDs, candidateMap, deletedBy, req.IncludeCrossServer)
		return
	}

	result := s.executeBulkDelete(r.Context(), req.CandidateIDs, candidateMap, deletedBy, req.IncludeCrossServer, nil)
	writeJSON(w, http.StatusOK, result)
}

type BulkDeleteProgress struct {
	Current   int    `json:"current"`
	Total     int    `json:"total"`
	Title     string `json:"title"`
	Status    string `json:"status"` // "deleting", "deleted", "failed", "skipped"
	Deleted   int    `json:"deleted"`
	Failed    int    `json:"failed"`
	Skipped   int    `json:"skipped"`
	TotalSize int64  `json:"total_size"`
}

func (s *Server) streamBulkDelete(w http.ResponseWriter, r *http.Request, candidateIDs []int64, candidateMap map[int64]models.MaintenanceCandidate, deletedBy string, includeCrossServer bool) {
	flusher, ok := sseFlusher(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	sendProgress := func(p BulkDeleteProgress) {
		data, err := json.Marshal(p)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	result := s.executeBulkDelete(r.Context(), candidateIDs, candidateMap, deletedBy, includeCrossServer, sendProgress)

	if finalData, err := json.Marshal(result); err == nil {
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", finalData)
		flusher.Flush()
	}
}

// executeBulkDelete performs the bulk delete with optional progress callback.
// Aborts early if the media server rejects consecutiveFailureLimit deletes in
// a row, since this typically indicates rate-limiting or a config issue.
func (s *Server) executeBulkDelete(ctx context.Context, candidateIDs []int64, candidateMap map[int64]models.MaintenanceCandidate, deletedBy string, includeCrossServer bool, onProgress func(BulkDeleteProgress)) models.BulkDeleteResult {
	const (
		basePacing              = 500 * time.Millisecond
		failurePacing           = 3 * time.Second
		consecutiveFailureLimit = 3
	)

	result := models.BulkDeleteResult{
		Errors: []models.BulkDeleteError{},
	}

	total := len(candidateIDs)

	// Track library item IDs already deleted so that two candidates pointing
	// at the same library item don't attempt a redundant media-server delete.
	deletedItemIDs := make(map[int64]bool)
	ruleCache := make(map[int64]*models.MaintenanceRule)
	consecutiveServerFailures := 0

	for i, candidateID := range candidateIDs {
		// Pace requests to avoid overwhelming media servers or reverse proxies.
		// Use a longer delay after a failed delete to let the server recover.
		if i > 0 {
			pacing := basePacing
			if consecutiveServerFailures > 0 {
				pacing = failurePacing
			}
			select {
			case <-ctx.Done():
				log.Printf("bulk delete cancelled: %v", ctx.Err())
				return result
			case <-time.After(pacing):
			}
		}

		candidate, exists := candidateMap[candidateID]
		title := "Unknown"
		if exists && candidate.Item != nil {
			title = candidate.Item.Title
		}

		if onProgress != nil {
			onProgress(BulkDeleteProgress{
				Current:   i + 1,
				Total:     total,
				Title:     title,
				Status:    "deleting",
				Deleted:   result.Deleted,
				Failed:    result.Failed,
				Skipped:   result.Skipped,
				TotalSize: result.TotalSize,
			})
		}

		if !exists {
			result.Failed++
			result.Errors = append(result.Errors, models.BulkDeleteError{
				CandidateID: candidateID,
				Title:       "Unknown",
				Error:       "candidate not found",
			})
			continue
		}

		if candidate.Item == nil {
			result.Failed++
			result.Errors = append(result.Errors, models.BulkDeleteError{
				CandidateID: candidateID,
				Title:       "Unknown",
				Error:       "library item not found",
			})
			continue
		}

		// If another candidate already deleted this library item (e.g. same item
		// flagged by two rules), count it as deleted without hitting the media server.
		if deletedItemIDs[candidate.LibraryItemID] {
			result.Deleted++
			result.TotalSize += candidate.Item.FileSize
			continue
		}

		// Re-check exclusions at delete time to prevent TOCTOU race condition
		// (item may have been excluded since user loaded the candidates list)
		excluded, err := s.store.IsItemExcluded(ctx, candidate.RuleID, candidate.LibraryItemID)
		if err != nil {
			log.Printf("check exclusion for candidate %d: %v", candidateID, err)
			result.Failed++
			result.Errors = append(result.Errors, models.BulkDeleteError{
				CandidateID: candidateID,
				Title:       candidate.Item.Title,
				Error:       "failed to verify exclusion status",
			})
			continue
		}
		if excluded {
			result.Skipped++
			continue
		}

		var delResult deleteItemResult
		if candidate.RuleID > 0 {
			rule, ok := ruleCache[candidate.RuleID]
			if !ok {
				fetchedRule, ruleErr := s.store.GetMaintenanceRule(ctx, candidate.RuleID)
				if ruleErr != nil {
					log.Printf("bulk delete: get rule %d: %v", candidate.RuleID, ruleErr)
				}
				rule = fetchedRule
				ruleCache[candidate.RuleID] = rule
			}
			if rule != nil && rule.CriterionType == models.CriterionKeepLatestSeasons {
				delResult = s.deleteOldSeasons(candidate, rule, deletedBy)
			} else {
				delResult = s.deleteItemFromServer(candidate, deletedBy)
			}
		} else {
			delResult = s.deleteItemFromServer(candidate, deletedBy)
		}

		if delResult.ServerDeleted {
			result.TotalSize += delResult.FileSize
		}

		if delResult.ServerDeleted && delResult.DBCleaned {
			result.Deleted++
			consecutiveServerFailures = 0
			deletedItemIDs[candidate.LibraryItemID] = true

			if includeCrossServer {
				s.deleteCrossServerCopies(candidate, deletedBy, deletedItemIDs, &result)
				if onProgress != nil {
					onProgress(BulkDeleteProgress{
						Current:   i + 1,
						Total:     total,
						Title:     title,
						Status:    "deleted",
						Deleted:   result.Deleted,
						Failed:    result.Failed,
						Skipped:   result.Skipped,
						TotalSize: result.TotalSize,
					})
				}
			}
		} else {
			log.Printf("bulk delete: %q (candidate %d): %s", candidate.Item.Title, candidateID, delResult.Error)
			result.Failed++
			result.Errors = append(result.Errors, models.BulkDeleteError{
				CandidateID: candidateID,
				Title:       candidate.Item.Title,
				Error:       delResult.Error,
			})
			if delResult.ServerDeleted {
				consecutiveServerFailures = 0
			} else {
				consecutiveServerFailures++
			}
			if consecutiveServerFailures >= consecutiveFailureLimit {
				remaining := total - i - 1
				log.Printf("bulk delete: aborting after %d consecutive failures, %d items remaining", consecutiveFailureLimit, remaining)
				result.Failed += remaining
				result.Errors = append(result.Errors, models.BulkDeleteError{
					Title: fmt.Sprintf("(%d items skipped)", remaining),
					Error: fmt.Sprintf("aborted: media server rejected %d consecutive deletes", consecutiveFailureLimit),
				})
				break
			}
		}
	}

	return result
}

func (s *Server) deleteCrossServerCopies(candidate models.MaintenanceCandidate, deletedBy string, deletedItemIDs map[int64]bool, result *models.BulkDeleteResult) {
	findCtx, findCancel := context.WithTimeout(context.Background(), 10*time.Second)
	matches, err := s.store.FindMatchingItems(findCtx, candidate.Item)
	findCancel()
	if err != nil {
		log.Printf("bulk delete: find cross-server copies for %q: %v", candidate.Item.Title, err)
		return
	}

	var didProcess bool
	for _, match := range matches {
		if deletedItemIDs[match.ID] {
			continue
		}
		if didProcess {
			time.Sleep(500 * time.Millisecond)
		}
		didProcess = true

		syntheticCandidate := models.MaintenanceCandidate{
			LibraryItemID: match.ID,
			Item:          &match,
		}
		crossResult := s.deleteItemFromServer(syntheticCandidate, deletedBy)

		if crossResult.ServerDeleted {
			result.TotalSize += crossResult.FileSize
		}
		if crossResult.ServerDeleted && crossResult.DBCleaned {
			result.Deleted++
			deletedItemIDs[match.ID] = true
		} else if crossResult.Error != "" {
			log.Printf("bulk delete: cross-server copy %q on server %d: %s", match.Title, match.ServerID, crossResult.Error)
			result.Failed++
			result.Errors = append(result.Errors, models.BulkDeleteError{
				CandidateID: candidate.ID,
				Title:       match.Title + " (cross-server)",
				Error:       crossResult.Error,
			})
		}
	}
}

// GET /api/maintenance/candidates/{id}/cross-server
func (s *Server) handleCrossServerItems(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id parameter")
		return
	}

	candidate, err := s.store.GetMaintenanceCandidate(r.Context(), id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "candidate not found")
		return
	}
	if err != nil {
		log.Printf("get candidate %d for cross-server: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to get candidate")
		return
	}

	if candidate.Item == nil {
		writeJSON(w, http.StatusOK, []models.LibraryItemCache{})
		return
	}

	matches, err := s.store.FindMatchingItems(r.Context(), candidate.Item)
	if err != nil {
		log.Printf("find matching items for candidate %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to find matching items")
		return
	}

	writeJSON(w, http.StatusOK, matches)
}
