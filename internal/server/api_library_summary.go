package server

import (
	"net/http"

	"streammon/internal/store"
)

type librarySummaryServerEntry struct {
	store.LibraryServerSummary
	ServerName string `json:"server_name"`
}

type librarySummaryResponse struct {
	TotalItems int                         `json:"total_items"`
	Movies     int                         `json:"movies"`
	Shows      int                         `json:"shows"`
	Episodes   int                         `json:"episodes"`
	Libraries  int                         `json:"libraries"`
	PerServer  []librarySummaryServerEntry `json:"per_server"`
}

func (s *Server) handleLibrarySummary(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.LibrarySummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	nameByID := make(map[int64]string, len(servers))
	for _, srv := range servers {
		nameByID[srv.ID] = srv.Name
	}

	resp := librarySummaryResponse{PerServer: make([]librarySummaryServerEntry, 0, len(entries))}
	for _, e := range entries {
		resp.TotalItems += e.TotalItems
		resp.Movies += e.Movies
		resp.Shows += e.Shows
		resp.Episodes += e.Episodes
		resp.Libraries += e.Libraries
		resp.PerServer = append(resp.PerServer, librarySummaryServerEntry{
			LibraryServerSummary: e,
			ServerName:           nameByID[e.ServerID],
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
