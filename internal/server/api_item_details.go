package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

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

	if strings.Contains(itemID, "..") || strings.Contains(itemID, "?") || strings.Contains(itemID, "#") {
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

	writeJSON(w, http.StatusOK, details)
}
