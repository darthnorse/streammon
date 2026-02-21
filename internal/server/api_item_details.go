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

	searchTitle := details.Title
	if details.SeriesTitle != "" {
		searchTitle = details.SeriesTitle
	}

	// Viewers can only see their own watch history
	userFilter := ""
	if user := UserFromContext(r.Context()); user != nil && user.Role == models.RoleViewer {
		userFilter = user.Name
	}
	history, err := s.store.HistoryForTitleByUser(searchTitle, userFilter, maxWatchHistoryEntries)
	if err != nil {
		log.Printf("WARN: HistoryForTitleByUser title=%q user=%q: %v", searchTitle, userFilter, err)
	}

	resp := itemDetailsResponse{
		ItemDetails:  details,
		WatchHistory: history,
	}
	writeJSON(w, http.StatusOK, resp)
}
