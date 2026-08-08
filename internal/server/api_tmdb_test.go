package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"streammon/internal/tmdb"
)

func newTestServerWithTMDB(t *testing.T, tmdbHandler http.Handler) (*testServer, *tmdb.Client) {
	t.Helper()
	mockSrv := httptest.NewServer(tmdbHandler)
	t.Cleanup(mockSrv.Close)

	srv, st := newTestServerWrapped(t)
	tc := tmdb.NewWithBaseURL("test-key", st, mockSrv.URL)
	srv.tmdbClient = tc
	return srv, tc
}

func TestTMDBSearch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := `{"page":1,"results":[{"id":1,"title":"Matrix"}]}`
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expected))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=matrix&page=1", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != expected {
			t.Fatalf("got %s, want %s", w.Body.String(), expected)
		}
	})

	t.Run("missing query returns 400", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestTMDBDiscover(t *testing.T) {
	t.Run("trending", func(t *testing.T) {
		expected := `{"page":1,"results":[]}`
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expected))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/discover/trending?page=1", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid category returns 404", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/discover/invalid", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestTMDBMovie(t *testing.T) {
	t.Run("returns envelope with library_items", func(t *testing.T) {
		tmdbData := `{"id":550,"title":"Fight Club"}`
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tmdbData))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movie/550", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var envelope tmdbEnvelope
		if err := json.NewDecoder(w.Body).Decode(&envelope); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(envelope.TMDB) != tmdbData {
			t.Fatalf("tmdb data mismatch: %s", envelope.TMDB)
		}
		if envelope.LibraryItems == nil {
			t.Fatal("library_items should be non-nil empty array")
		}
	})

	t.Run("invalid ID returns 400", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/movie/abc", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestTMDBPerson(t *testing.T) {
	expected := `{"id":6789,"name":"Actor"}`
	srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(expected))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/person/6789", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != expected {
		t.Fatalf("got %s, want %s", w.Body.String(), expected)
	}
}

func TestTMDBCollection(t *testing.T) {
	expected := `{"id":131295,"name":"Collection","parts":[]}`
	srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(expected))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/collection/131295", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != expected {
		t.Fatalf("got %s, want %s", w.Body.String(), expected)
	}
}

func TestTMDBTVStatuses(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/tv/123") {
				w.Write([]byte(`{"id":123,"name":"Test Show","status":"Ended"}`))
			} else if strings.Contains(r.URL.Path, "/tv/456") {
				w.Write([]byte(`{"id":456,"name":"Another Show","status":"Returning Series"}`))
			}
		}))

		body := `{"tmdb_ids":[123,456]}`
		req := httptest.NewRequest(http.MethodPost, "/api/tmdb/tv/statuses", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if result["123"] != "Ended" {
			t.Fatalf("expected 123=Ended, got %v", result["123"])
		}
		if result["456"] != "Returning Series" {
			t.Fatalf("expected 456=Returning Series, got %v", result["456"])
		}
	})

	t.Run("empty IDs returns empty map", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		body := `{"tmdb_ids":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/tmdb/tv/statuses", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty result, got %v", result)
		}
	})

	t.Run("too many IDs returns 400", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		ids := make([]string, 101)
		for i := range ids {
			ids[i] = strconv.Itoa(i)
		}
		body := `{"tmdb_ids":[` + strings.Join(ids, ",") + `]}`
		req := httptest.NewRequest(http.MethodPost, "/api/tmdb/tv/statuses", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("nil TMDB client returns 503", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"tmdb_ids":[123]}`
		req := httptest.NewRequest(http.MethodPost, "/api/tmdb/tv/statuses", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// captureDiscoverQuery serves any TMDB request with the given body and records
// the last path+query the handler produced.
func captureDiscoverQuery(t *testing.T, body string) (*testServer, func() (string, url.Values)) {
	t.Helper()
	var mu sync.Mutex
	var path string
	var query url.Values
	srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		path = r.URL.Path
		query = r.URL.Query()
		mu.Unlock()
		w.Write([]byte(body))
	}))
	return srv, func() (string, url.Values) {
		mu.Lock()
		defer mu.Unlock()
		return path, query
	}
}

const discoverStubBody = `{"page":1,"results":[{"id":1,"title":"X"}],"total_pages":1}`

func getDiscover(t *testing.T, srv *testServer, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestDiscoverUnfilteredUsesCuratedEndpoints(t *testing.T) {
	cases := map[string]string{
		"trending":        "/trending/all/week",
		"movies":          "/movie/popular",
		"movies/upcoming": "/movie/upcoming",
		"tv":              "/tv/popular",
		"tv/upcoming":     "/tv/on_the_air",
	}
	for category, wantPath := range cases {
		srv, captured := captureDiscoverQuery(t, discoverStubBody)
		w := getDiscover(t, srv, "/api/tmdb/discover/"+category+"?page=1")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", category, w.Code, w.Body.String())
		}
		path, _ := captured()
		if path != wantPath {
			t.Fatalf("%s: got upstream path %s, want %s", category, path, wantPath)
		}
	}
}

// Trending is the one category whose body is passed through untouched. Odd
// spacing and key order prove no re-marshalling happens.
func TestDiscoverTrendingUnfilteredIsByteIdentical(t *testing.T) {
	body := `{"results":[ {"media_type":"movie","id":1} ],  "page":1}`
	srv, _ := captureDiscoverQuery(t, body)

	w := getDiscover(t, srv, "/api/tmdb/discover/trending?page=1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != body {
		t.Fatalf("body was rewritten:\n got %s\nwant %s", w.Body.String(), body)
	}
}

func TestDiscoverFilteredSwitchesToDiscoverEndpoint(t *testing.T) {
	srv, captured := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/tv?year=2024&genres=80,18&sort=rating&rating=7")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	path, q := captured()
	if path != "/discover/tv" {
		t.Fatalf("got upstream path %s, want /discover/tv", path)
	}
	if q.Get("first_air_date.gte") != "2024-01-01" {
		t.Fatalf("year: got %q", q.Get("first_air_date.gte"))
	}
	if q.Get("with_genres") != "80,18" {
		t.Fatalf("genres: got %q", q.Get("with_genres"))
	}
	if q.Get("sort_by") != "vote_average.desc" {
		t.Fatalf("sort: got %q", q.Get("sort_by"))
	}
	if q.Get("vote_average.gte") != "7" {
		t.Fatalf("rating: got %q", q.Get("vote_average.gte"))
	}
}

func TestDiscoverFilteredInjectsMediaType(t *testing.T) {
	srv, _ := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/trending?type=movie&year=2024")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"media_type":"movie"`) {
		t.Fatalf("expected media_type injected, got %s", w.Body.String())
	}
}

func TestDiscoverTrendingTypeAloneIsAFilter(t *testing.T) {
	srv, captured := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/trending?type=movie")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	path, _ := captured()
	if path != "/discover/movie" {
		t.Fatalf("type alone must filter: got upstream path %s", path)
	}
}

func TestDiscoverTrendingFiltersRequireType(t *testing.T) {
	srv, _ := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/trending?year=2024")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDiscoverTypeRejectedOnNonTrending(t *testing.T) {
	srv, _ := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/tv?type=movie")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDiscoverUpcomingRejectsYear(t *testing.T) {
	for _, category := range []string{"movies/upcoming", "tv/upcoming"} {
		srv, _ := captureDiscoverQuery(t, discoverStubBody)
		w := getDiscover(t, srv, "/api/tmdb/discover/"+category+"?year=2024")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", category, w.Code)
		}
	}
}

func TestDiscoverUpcomingFilteredKeepsFutureFloor(t *testing.T) {
	cases := []struct{ category, path, field string }{
		{"movies/upcoming", "/discover/movie", "primary_release_date.gte"},
		{"tv/upcoming", "/discover/tv", "air_date.gte"},
	}
	for _, tc := range cases {
		srv, captured := captureDiscoverQuery(t, discoverStubBody)
		w := getDiscover(t, srv, "/api/tmdb/discover/"+tc.category+"?genres=28")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", tc.category, w.Code, w.Body.String())
		}
		path, q := captured()
		if path != tc.path {
			t.Fatalf("%s: got upstream path %s, want %s", tc.category, path, tc.path)
		}
		if q.Get(tc.field) == "" {
			t.Fatalf("%s: expected a %s floor, query was %v", tc.category, tc.field, q)
		}
	}
}

func TestDiscoverRejectsInvalidFilters(t *testing.T) {
	cases := []string{
		"/api/tmdb/discover/tv?sort=alphabetical",
		"/api/tmdb/discover/tv?genres=drama",
		"/api/tmdb/discover/tv?genres=80,,18",
		"/api/tmdb/discover/tv?genres=80,18,35,28,12,99",
		"/api/tmdb/discover/tv?rating=11",
		"/api/tmdb/discover/tv?rating=0",
		"/api/tmdb/discover/tv?rating=abc",
		// ParseFloat accepts NaN, and NaN fails every ordinary bounds comparison.
		"/api/tmdb/discover/tv?rating=NaN",
		"/api/tmdb/discover/tv?year=1500",
		"/api/tmdb/discover/tv?year=abc",
		// A future floor is unsatisfiable and, with sort=newest, self-contradictory.
		"/api/tmdb/discover/tv?year=2999",
		"/api/tmdb/discover/trending?type=person&year=2024",
	}
	for _, target := range cases {
		srv, _ := captureDiscoverQuery(t, discoverStubBody)
		w := getDiscover(t, srv, target)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, w.Code)
		}
	}
}

// popularity is the default, so naming it explicitly narrows nothing.
func TestDiscoverExplicitPopularitySortStaysCurated(t *testing.T) {
	srv, captured := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/tv?sort=popularity")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	path, _ := captured()
	if path != "/tv/popular" {
		t.Fatalf("got upstream path %s, want /tv/popular", path)
	}
}

func TestDiscoverUnknownCategoryStill404(t *testing.T) {
	srv, _ := captureDiscoverQuery(t, discoverStubBody)
	w := getDiscover(t, srv, "/api/tmdb/discover/nope?year=2024")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMovieGenresEndpoint(t *testing.T) {
	expected := `{"genres":[{"id":80,"name":"Crime"}]}`
	srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/genre/movie/list" {
			t.Errorf("got path %s", r.URL.Path)
		}
		w.Write([]byte(expected))
	}))

	w := getDiscover(t, srv, "/api/tmdb/genres/movie")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != expected {
		t.Fatalf("got %s", w.Body.String())
	}
}

func TestTMDBNoClient(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no TMDB client, got %d", w.Code)
	}
}
