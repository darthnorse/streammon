package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

type childrenSeasonsResponse struct {
	Seasons []models.Season `json:"seasons"`
}

type childrenEpisodesResponse struct {
	Episodes []models.Episode `json:"episodes"`
}

func (s *Server) handleGetChildren(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}
	itemID := chi.URLParam(r, "*")
	if itemID == "" || !isValidPathSegment(itemID) {
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
		log.Printf("children: GetItemDetails server=%d item=%s: %v", serverID, itemID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch parent")
		return
	}

	switch details.Level {
	case "show":
		seasons, err := ms.GetSeasons(r.Context(), itemID)
		if err != nil {
			log.Printf("children: GetSeasons server=%d show=%s: %v", serverID, itemID, err)
			writeError(w, http.StatusInternalServerError, "failed to fetch seasons")
			return
		}
		if seasons == nil {
			seasons = []models.Season{}
		}
		writeJSON(w, http.StatusOK, childrenSeasonsResponse{Seasons: seasons})
	case "season":
		episodes, err := ms.GetEpisodes(r.Context(), itemID)
		if err != nil {
			log.Printf("children: GetEpisodes server=%d season=%s: %v", serverID, itemID, err)
			writeError(w, http.StatusInternalServerError, "failed to fetch episodes")
			return
		}
		if episodes == nil {
			episodes = []models.Episode{}
		}
		writeJSON(w, http.StatusOK, childrenEpisodesResponse{Episodes: episodes})
	default:
		writeError(w, http.StatusBadRequest, "no children for this item")
	}
}
