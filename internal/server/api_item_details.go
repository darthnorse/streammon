package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

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

	searchTitle := details.Title
	if details.SeriesTitle != "" {
		searchTitle = details.SeriesTitle
	}
	history, _ := s.store.HistoryForTitle(searchTitle, 10)

	resp := itemDetailsResponse{
		ItemDetails:  details,
		WatchHistory: history,
	}
	writeJSON(w, http.StatusOK, resp)
}
