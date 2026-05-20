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

	userFilter := ""
	if user := UserFromContext(r.Context()); user != nil && user.Role == models.RoleViewer {
		userFilter = user.Name
	}

	level := details.Level
	historyKey := itemID
	switch level {
	case "season", "episode":
		historyKey = details.SeriesID
		if historyKey == "" {
			historyKey = details.ParentID
		}
	case "movie", "":
		level = "movie"
	}

	fallbackKey := historyKey
	if level == "episode" {
		fallbackKey = itemID
	}

	// historyKey is the parent show id for season/episode, else the item itself —
	// used both for resolving tmdb_id and (for non-episode levels) for the single-server fallback.
	tmdbID, err := s.store.GetLibraryItemTMDBID(r.Context(), serverID, historyKey)
	if err != nil {
		log.Printf("WARN: GetLibraryItemTMDBID server=%d key=%s: %v", serverID, historyKey, err)
	}
	if tmdbID != "" {
		details.TMDBID = tmdbID
	}

	// Title-based fallback for legacy rows lacking a library_items match.
	if details.TMDBID == "" && details.SeriesTitle != "" {
		if fallbackID, err := s.store.GetLibraryItemTMDBIDByTitle(r.Context(), serverID, details.SeriesTitle, string(models.MediaTypeTV)); err != nil {
			log.Printf("WARN: GetLibraryItemTMDBIDByTitle server=%d title=%q: %v", serverID, details.SeriesTitle, err)
		} else if fallbackID != "" {
			details.TMDBID = fallbackID
			tmdbID = fallbackID
		}
	}

	var history []models.WatchHistoryEntry
	if tmdbID != "" {
		matches, mErr := s.store.FindLibraryItemsByTMDBID(r.Context(), tmdbID)
		if mErr != nil {
			log.Printf("WARN: FindLibraryItemsByTMDBID tmdb=%s: %v", tmdbID, mErr)
		} else if len(matches) > 0 {
			h, hErr := s.store.HistoryForItemAcrossServers(r.Context(), matches, level, details.SeasonNumber, details.EpisodeNumber, userFilter, maxWatchHistoryEntries)
			if hErr != nil {
				log.Printf("WARN: HistoryForItemAcrossServers tmdb=%s: %v", tmdbID, hErr)
			} else {
				history = h
			}
		}
	}

	// Single-server fallback. Intentionally also runs when cross-server returned zero —
	// for episode level this can recover rows whose grandparent_item_id has changed after
	// a library re-sync but whose item_id is still tracked in watch_history.
	if len(history) == 0 {
		fallback, fbErr := s.store.HistoryForItem(serverID, fallbackKey, level, details.SeasonNumber, userFilter, maxWatchHistoryEntries)
		if fbErr != nil {
			log.Printf("WARN: HistoryForItem server=%d item=%s level=%s: %v", serverID, fallbackKey, level, fbErr)
		} else {
			history = fallback
		}
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
