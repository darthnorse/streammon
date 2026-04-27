package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleTMDBSeason(w http.ResponseWriter, r *http.Request) {
	tmdbID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}
	seasonNumber, err := strconv.Atoi(chi.URLParam(r, "seasonNumber"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid season number")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.GetTVSeason(ctx, tmdbID, seasonNumber)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}
