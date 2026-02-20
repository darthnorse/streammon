package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/store"
)

const tmdbTimeout = 15 * time.Second

func (s *Server) tmdbRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.tmdbClient == nil {
			writeError(w, http.StatusServiceUnavailable, "TMDB not available")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parsePage(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	return page
}

func (s *Server) handleTMDBSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.Search(ctx, query, parsePage(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleTMDBDiscover(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "*")

	dispatchers := map[string]func(context.Context, int) (json.RawMessage, error){
		"trending":        s.tmdbClient.Trending,
		"movies":          s.tmdbClient.PopularMovies,
		"movies/upcoming": s.tmdbClient.UpcomingMovies,
		"tv":              s.tmdbClient.PopularTV,
		"tv/upcoming":     s.tmdbClient.UpcomingTV,
	}

	fn, ok := dispatchers[category]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown discover category")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := fn(ctx, parsePage(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	if category != "trending" {
		mediaType := "movie"
		if strings.HasPrefix(category, "tv") {
			mediaType = "tv"
		}
		data = injectMediaType(data, mediaType)
	}

	writeRawJSON(w, http.StatusOK, data)
}

func injectMediaType(raw json.RawMessage, mediaType string) json.RawMessage {
	var page struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return raw
	}

	mtBytes, _ := json.Marshal(mediaType)
	for i, item := range page.Results {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(item, &obj); err != nil {
			continue
		}
		obj["media_type"] = mtBytes
		if modified, err := json.Marshal(obj); err == nil {
			page.Results[i] = modified
		}
	}

	var full map[string]json.RawMessage
	if err := json.Unmarshal(raw, &full); err != nil {
		return raw
	}
	if resultsBytes, err := json.Marshal(page.Results); err == nil {
		full["results"] = resultsBytes
	}
	if out, err := json.Marshal(full); err == nil {
		return out
	}
	return raw
}

type tmdbEnvelope struct {
	TMDB         json.RawMessage     `json:"tmdb"`
	LibraryItems []store.LibraryMatch `json:"library_items"`
}

func (s *Server) handleTMDBMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.GetMovie(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	s.writeTMDBEnvelope(w, r, data, strconv.Itoa(id))
}

func (s *Server) handleTMDBTV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.GetTV(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	s.writeTMDBEnvelope(w, r, data, strconv.Itoa(id))
}

func (s *Server) writeTMDBEnvelope(w http.ResponseWriter, r *http.Request, tmdbData json.RawMessage, tmdbID string) {
	matches, err := s.store.FindLibraryItemsByTMDBID(r.Context(), tmdbID)
	if err != nil {
		matches = nil
	}
	if matches == nil {
		matches = []store.LibraryMatch{}
	}

	writeJSON(w, http.StatusOK, tmdbEnvelope{
		TMDB:         tmdbData,
		LibraryItems: matches,
	})
}

func (s *Server) handleTMDBPerson(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid person ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.GetPerson(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleTMDBCollection(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := s.tmdbClient.GetCollection(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleLibraryTMDBIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := s.store.GetLibraryTMDBIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch library IDs")
		return
	}
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"ids": ids})
}
