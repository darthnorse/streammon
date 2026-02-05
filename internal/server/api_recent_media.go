package server

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"streammon/internal/models"
)

func (s *Server) handleGetRecentMedia(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	var allItems []models.LibraryItem

	for _, srv := range servers {
		if !srv.Enabled || !srv.ShowRecentMedia {
			continue
		}

		ms, ok := s.poller.GetServer(srv.ID)
		if !ok {
			continue
		}

		items, err := ms.GetRecentlyAdded(r.Context(), 25)
		if err != nil {
			log.Printf("recent media from %s: %v", ms.Name(), err)
			continue
		}

		allItems = append(allItems, items...)
	}

	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].AddedAt.After(allItems[j].AddedAt)
	})

	allItems = dedupeLibraryItems(allItems)

	if len(allItems) > 50 {
		allItems = allItems[:50]
	}

	writeJSON(w, http.StatusOK, allItems)
}

func dedupeLibraryItems(items []models.LibraryItem) []models.LibraryItem {
	seen := make(map[string]bool)
	result := make([]models.LibraryItem, 0, len(items))

	for _, item := range items {
		keys := fallbackDedupeKeys(item)
		extKey := item.ExternalIDs.DedupeKey()

		isDupe := false
		for _, k := range keys {
			if seen[k] {
				isDupe = true
				break
			}
		}
		if extKey != "" && seen[extKey] {
			isDupe = true
		}

		if isDupe {
			continue
		}

		for _, k := range keys {
			seen[k] = true
		}
		if extKey != "" {
			seen[extKey] = true
		}

		result = append(result, item)
	}

	return result
}

func fallbackDedupeKeys(item models.LibraryItem) []string {
	if item.MediaType == models.MediaTypeTV {
		return []string{fmt.Sprintf("%s:s%de%d", item.Title, item.SeasonNumber, item.EpisodeNumber)}
	}
	// For movies, track both title-only and title+year to handle servers that don't return year
	if item.Year == 0 {
		return []string{item.Title}
	}
	return []string{item.Title, fmt.Sprintf("%s:%d", item.Title, item.Year)}
}
