package server

import (
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

	if len(allItems) > 50 {
		allItems = allItems[:50]
	}

	writeJSON(w, http.StatusOK, allItems)
}
