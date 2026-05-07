package server

import (
	"net/http"

	"streammon/internal/store"
)

type librarySummaryResponse struct {
	TotalItems int                          `json:"total_items"`
	Movies     int                          `json:"movies"`
	Shows      int                          `json:"shows"`
	Episodes   int                          `json:"episodes"`
	Other      int                          `json:"other"`
	Libraries  int                          `json:"libraries"`
	PerServer  []store.LibraryServerSummary `json:"per_server"`
}

func (s *Server) handleLibrarySummary(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.LibrarySummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp := librarySummaryResponse{PerServer: make([]store.LibraryServerSummary, 0, len(entries))}
	for _, e := range entries {
		resp.TotalItems += e.TotalItems
		resp.Movies += e.Movies
		resp.Shows += e.Shows
		resp.Episodes += e.Episodes
		resp.Other += e.Other
		resp.Libraries += e.Libraries
		resp.PerServer = append(resp.PerServer, e)
	}
	writeJSON(w, http.StatusOK, resp)
}
