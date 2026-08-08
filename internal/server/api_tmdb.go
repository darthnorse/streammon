package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tmdb"
)

const tmdbTimeout = 15 * time.Second

func (s *Server) discoverAccessBlocked(w http.ResponseWriter, r *http.Request) bool {
	if user := UserFromContext(r.Context()); user != nil && user.Role != models.RoleAdmin {
		allowed, err := s.store.GetGuestSetting("show_discover")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return true
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "discover access disabled")
			return true
		}
	}
	return false
}

func (s *Server) requireDiscover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.discoverAccessBlocked(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) tmdbRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.tmdbClient == nil {
			writeError(w, http.StatusServiceUnavailable, "TMDB not available")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// maxDiscoverPage mirrors TMDB's own ceiling: it rejects requests for pages
// beyond 500. Without a clamp here, page is an unbounded, client-controlled
// input into the cache key.
const maxDiscoverPage = 500

func parsePage(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > maxDiscoverPage {
		page = maxDiscoverPage
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

type discoverCategory struct {
	fn        func(context.Context, int, string) (json.RawMessage, error)
	mediaType string // "" means the category mixes types and requires ?type=
	upcoming  bool
}

const (
	minDiscoverYear = 1900
	maxGenreFilters = 5
)

func (s *Server) discoverCategories() map[string]discoverCategory {
	return map[string]discoverCategory{
		"trending":        {fn: s.tmdbClient.Trending},
		"movies":          {fn: s.tmdbClient.PopularMovies, mediaType: "movie"},
		"movies/upcoming": {fn: s.tmdbClient.UpcomingMovies, mediaType: "movie", upcoming: true},
		"tv":              {fn: s.tmdbClient.PopularTV, mediaType: "tv"},
		"tv/upcoming":     {fn: s.tmdbClient.UpcomingTV, mediaType: "tv", upcoming: true},
	}
}

// parseDiscoverFilters reads the filter query parameters. active reports whether
// any filter was supplied, which is what selects the /discover endpoint over the
// curated one.
func parseDiscoverFilters(r *http.Request, cat discoverCategory) (filters tmdb.DiscoverFilters, active bool, err error) {
	q := r.URL.Query()

	if raw := q.Get("year"); raw != "" {
		if cat.upcoming {
			return filters, false, fmt.Errorf("year is not supported on upcoming categories")
		}
		// A future floor cannot be satisfied on a non-upcoming category, and
		// combined with sort=newest it would produce gte=<future>, lte=<today>.
		year, convErr := strconv.Atoi(raw)
		if convErr != nil || year < minDiscoverYear || year > time.Now().UTC().Year() {
			return filters, false, fmt.Errorf("invalid year")
		}
		filters.YearGTE = year
		active = true
	}

	if raw := q.Get("genres"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > maxGenreFilters {
			return filters, false, fmt.Errorf("too many genres (max %d)", maxGenreFilters)
		}
		for _, part := range parts {
			id, convErr := strconv.Atoi(part)
			if convErr != nil || id <= 0 {
				return filters, false, fmt.Errorf("invalid genre id")
			}
			filters.GenreIDs = append(filters.GenreIDs, id)
		}
		active = true
	}

	// popularity is the default ordering, so an explicit sort=popularity narrows
	// nothing and must not switch away from the curated endpoint. The client
	// omits it for the same reason; this keeps a hand-edited URL consistent.
	if raw := q.Get("sort"); raw != "" {
		switch raw {
		case "popularity":
		case "rating", "newest":
			filters.Sort = raw
			active = true
		default:
			return filters, false, fmt.Errorf("invalid sort")
		}
	}

	// A zero floor is not a filter; the client omits the parameter for "any".
	// The positive form of the bounds check also rejects NaN, which ParseFloat
	// accepts and which fails every ordinary comparison.
	if raw := q.Get("rating"); raw != "" {
		rating, convErr := strconv.ParseFloat(raw, 64)
		if convErr != nil || !(rating > 0 && rating <= 10) {
			return filters, false, fmt.Errorf("invalid rating")
		}
		filters.RatingGTE = rating
		active = true
	}

	filters.Upcoming = cat.upcoming
	return filters, active, nil
}

func (s *Server) handleTMDBDiscover(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "*")
	cat, ok := s.discoverCategories()[category]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown discover category")
		return
	}

	region, err := s.store.GetDiscoverRegion()
	if err != nil {
		log.Printf("failed to read discover region setting: %v", err)
	}

	mediaType := cat.mediaType
	typeParam := r.URL.Query().Get("type")
	if typeParam != "" {
		if cat.mediaType != "" {
			writeError(w, http.StatusBadRequest, "type is only valid on mixed categories")
			return
		}
		if typeParam != "movie" && typeParam != "tv" {
			writeError(w, http.StatusBadRequest, "invalid type")
			return
		}
		mediaType = typeParam
	}

	filters, filtersActive, err := parseDiscoverFilters(r, cat)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Narrowing a mixed category to one media type is itself a filter.
	active := filtersActive || typeParam != ""
	if active && mediaType == "" {
		writeError(w, http.StatusBadRequest, "type is required when filtering this category")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	var data json.RawMessage
	switch {
	case filtersActive:
		filters.Region = region
		filters.Now = time.Now().UTC()
		data, err = s.tmdbClient.Discover(ctx, mediaType, filters, parsePage(r))
	case cat.mediaType == "" && typeParam != "":
		// A mixed category narrowed only by ?type= keeps its trending ranking
		// instead of falling back to Discover's all-time popularity sort.
		data, err = s.tmdbClient.TrendingByType(ctx, mediaType, parsePage(r))
	default:
		data, err = cat.fn(ctx, parsePage(r), region)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	if mediaType != "" {
		data = injectMediaType(data, mediaType)
	}

	writeRawJSON(w, http.StatusOK, data)
}

// serveTMDBRaw runs a no-arg TMDB client call under the standard timeout and
// passes its result straight through, mapping any error to a 502.
func (s *Server) serveTMDBRaw(w http.ResponseWriter, r *http.Request, fetch func(context.Context) (json.RawMessage, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	data, err := fetch(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleTMDBMovieGenres(w http.ResponseWriter, r *http.Request) {
	s.serveTMDBRaw(w, r, func(ctx context.Context) (json.RawMessage, error) {
		return s.tmdbClient.GetGenres(ctx, "movie")
	})
}

func (s *Server) handleTMDBRegions(w http.ResponseWriter, r *http.Request) {
	s.serveTMDBRaw(w, r, s.tmdbClient.Regions)
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
	TMDB         json.RawMessage      `json:"tmdb"`
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
		log.Printf("WARN: FindLibraryItemsByTMDBID tmdb=%s: %v", tmdbID, err)
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

func (s *Server) handleTMDBTVStatuses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TMDBIDs []int `json:"tmdb_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.TMDBIDs) > 100 {
		writeError(w, http.StatusBadRequest, "too many IDs (max 100)")
		return
	}
	if len(req.TMDBIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tmdbTimeout)
	defer cancel()

	var mu sync.Mutex
	result := make(map[string]string, len(req.TMDBIDs))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for _, id := range req.TMDBIDs {
		g.Go(func() error {
			data, err := s.tmdbClient.GetTV(ctx, id)
			if err != nil {
				log.Printf("tmdb tv status %d: %v", id, err)
				return nil
			}
			var parsed struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(data, &parsed); err == nil && parsed.Status != "" {
				mu.Lock()
				result[strconv.Itoa(id)] = parsed.Status
				mu.Unlock()
			}
			return nil
		})
	}

	_ = g.Wait()

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTMDBTVGenres(w http.ResponseWriter, r *http.Request) {
	s.serveTMDBRaw(w, r, func(ctx context.Context) (json.RawMessage, error) {
		return s.tmdbClient.GetGenres(ctx, "tv")
	})
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
