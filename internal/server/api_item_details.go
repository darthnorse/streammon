package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

const maxWatchHistoryEntries = 10

type itemDetailsResponse struct {
	*models.ItemDetails
	WatchHistory []models.WatchHistoryEntry `json:"watch_history,omitempty"`
}

func (s *Server) handleGetItemDetails(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	itemID := chi.URLParam(r, "*")
	if itemID == "" {
		writeError(w, http.StatusBadRequest, "missing item id")
		return
	}

	if !isValidPathSegment(itemID) {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	ms, ok := s.poller.GetServer(serverID)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	details, err := ms.GetItemDetails(r.Context(), itemID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch item details")
		return
	}

	if tmdbID, err := s.store.GetLibraryItemTMDBID(r.Context(), serverID, itemID); err != nil {
		log.Printf("WARN: GetLibraryItemTMDBID server=%d item=%s: %v", serverID, itemID, err)
	} else if tmdbID != "" {
		details.TMDBID = tmdbID
	}

	if details.TMDBID == "" && details.SeriesTitle != "" {
		if tmdbID, err := s.store.GetLibraryItemTMDBIDByTitle(r.Context(), serverID, details.SeriesTitle, string(models.MediaTypeTV)); err != nil {
			log.Printf("WARN: GetLibraryItemTMDBIDByTitle server=%d title=%q: %v", serverID, details.SeriesTitle, err)
		} else if tmdbID != "" {
			details.TMDBID = tmdbID
		}
	}

	userFilter := ""
	if user := UserFromContext(r.Context()); user != nil && user.Role == models.RoleViewer {
		userFilter = user.Name
	}

	level := details.Level
	historyKey := itemID
	switch level {
	case "season":
		historyKey = details.SeriesID
		if historyKey == "" {
			historyKey = details.ParentID
		}
	case "movie", "":
		level = "movie"
	}

	history, err := s.store.HistoryForItem(serverID, historyKey, level, details.SeasonNumber, userFilter, maxWatchHistoryEntries)
	if err != nil {
		log.Printf("WARN: HistoryForItem server=%d item=%s level=%s: %v", serverID, historyKey, level, err)
	}

	// Fallback to title match for old rows missing grandparent_item_id (show/season only — needs a reliable show title).
	if len(history) == 0 && (level == "show" || (level == "season" && details.SeriesTitle != "")) {
		searchTitle := details.SeriesTitle
		if searchTitle == "" {
			searchTitle = details.Title
		}
		if searchTitle != "" {
			fallback, fbErr := s.store.HistoryForTitleByUser(serverID, searchTitle, userFilter, maxWatchHistoryEntries)
			if fbErr != nil {
				log.Printf("WARN: HistoryForTitleByUser fallback title=%q: %v", searchTitle, fbErr)
			} else {
				history = fallback
			}
		}
	}

	resp := itemDetailsResponse{
		ItemDetails:  details,
		WatchHistory: history,
	}
	writeJSON(w, http.StatusOK, resp)
}
