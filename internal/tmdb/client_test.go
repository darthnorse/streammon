package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"streammon/internal/httputil"
	"streammon/internal/store"
)

func migrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "migrations")
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	dir := migrationsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

func newTestClient(t *testing.T, handler http.Handler, s *store.Store) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		apiKey:  "test-key",
		baseURL: srv.URL,
		http:    srv.Client(),
		store:   s,
		limiter: rate.NewLimiter(rate.Inf, 0),
	}
}

func TestSearchCacheMiss(t *testing.T) {
	expected := json.RawMessage(`{"page":1,"results":[{"id":1,"title":"Matrix"}]}`)
	var calls int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Errorf("expected api_key=test-key, got %s", r.URL.Query().Get("api_key"))
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.Search(context.Background(), "matrix", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
	if calls != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", calls)
	}
}

func TestSearchCacheHit(t *testing.T) {
	s := newTestStore(t)
	cached := json.RawMessage(`{"page":1,"results":[],"total_results":0}`)
	if err := s.SetCachedTMDB("search:matrix:1", cached); err != nil {
		t.Fatal(err)
	}

	var calls int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"should":"not be called"}`))
	}), s)

	data, err := c.Search(context.Background(), "matrix", 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(cached) {
		t.Fatalf("got %s, want %s", data, cached)
	}
	if calls != 0 {
		t.Fatalf("expected 0 HTTP calls (cache hit), got %d", calls)
	}
}

func TestCacheExpiry(t *testing.T) {
	s := newTestStore(t)
	old := json.RawMessage(`{"old":true}`)
	if err := s.SetCachedTMDB("movie:123", old); err != nil {
		t.Fatal(err)
	}
	s.BackdateTMDBCache("movie:123", time.Now().UTC().Add(-25*time.Hour))

	fresh := json.RawMessage(`{"fresh":true}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fresh)
	}), s)

	data, err := c.GetMovie(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(fresh) {
		t.Fatalf("expected fresh data after expiry, got %s", data)
	}
}

func TestDoRedactsAPIKeyOnFailure(t *testing.T) {
	// Server is closed before use, so the request fails with a connection
	// error (*url.Error) whose message embeds the full request URL.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	secret := "SUPERSECRETTMDBKEY"
	c := &Client{
		apiKey:  secret,
		baseURL: ts.URL,
		http:    httputil.NewClient(),
		limiter: rate.NewLimiter(rate.Inf, 0),
	}

	_, err := c.do(context.Background(), "/configuration", nil)
	if err == nil {
		t.Fatal("expected an error from a closed server")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("api key leaked into error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status_message":"not found"}`))
	}), newTestStore(t))

	_, err := c.GetMovie(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestNilStore(t *testing.T) {
	expected := json.RawMessage(`{"id":1}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(expected)
	}))
	t.Cleanup(srv.Close)

	c := &Client{apiKey: "test-key", baseURL: srv.URL, http: srv.Client(), store: nil, limiter: rate.NewLimiter(rate.Inf, 0)}

	data, err := c.GetMovie(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestGetPerson(t *testing.T) {
	expected := json.RawMessage(`{"id":6789,"name":"Actor","combined_credits":{"cast":[]}}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/person/6789" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("append_to_response") != "combined_credits" {
			t.Error("expected append_to_response=combined_credits")
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.GetPerson(context.Background(), 6789)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestGetCollection(t *testing.T) {
	expected := json.RawMessage(`{"id":131295,"name":"Test Collection","parts":[]}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collection/131295" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.GetCollection(context.Background(), 131295)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestTrending(t *testing.T) {
	expected := json.RawMessage(`{"page":1,"results":[]}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trending/all/week" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.Trending(context.Background(), 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestTrendingIgnoresRegion(t *testing.T) {
	expected := json.RawMessage(`{"page":1,"results":[]}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trending/all/week" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("region"); got != "" {
			t.Errorf("expected no region param, got %q", got)
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.Trending(context.Background(), 1, "US")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func TestTrendingByType(t *testing.T) {
	cases := []struct{ mediaType, wantPath string }{
		{"movie", "/trending/movie/week"},
		{"tv", "/trending/tv/week"},
	}
	for _, tc := range cases {
		expected := json.RawMessage(`{"page":1,"results":[]}`)
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != tc.wantPath {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("region"); got != "" {
				t.Errorf("expected no region param, got %q", got)
			}
			w.Write(expected)
		}), newTestStore(t))

		data, err := c.TrendingByType(context.Background(), tc.mediaType, 1)
		if err != nil {
			t.Fatalf("%s: TrendingByType: %v", tc.mediaType, err)
		}
		if string(data) != string(expected) {
			t.Fatalf("%s: got %s, want %s", tc.mediaType, data, expected)
		}
	}
}

func TestTrendingByTypeRejectsUnknownMediaType(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be called")
	}), newTestStore(t))

	if _, err := c.TrendingByType(context.Background(), "person", 1); err == nil {
		t.Fatal("expected error for media type person")
	}
}

func TestRegions(t *testing.T) {
	expected := json.RawMessage(`[{"iso_3166_1":"US","english_name":"United States"}]`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configuration/countries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write(expected)
	}), newTestStore(t))

	data, err := c.Regions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}

func discoverQueryFor(t *testing.T, mediaType string, f DiscoverFilters, page int) (string, url.Values) {
	t.Helper()
	var gotPath string
	var gotQuery url.Values
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"page":1,"results":[]}`))
	}), newTestStore(t))

	if _, err := c.Discover(context.Background(), mediaType, f, page); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return gotPath, gotQuery
}

var testNow = time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

func TestDiscoverPathPerMediaType(t *testing.T) {
	for _, mt := range []string{"movie", "tv"} {
		path, _ := discoverQueryFor(t, mt, DiscoverFilters{Now: testNow}, 1)
		if path != "/discover/"+mt {
			t.Fatalf("mediaType %s: got path %s", mt, path)
		}
	}
}

func TestDiscoverRejectsUnknownMediaType(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream must not be called")
	}), newTestStore(t))

	if _, err := c.Discover(context.Background(), "person", DiscoverFilters{Now: testNow}, 1); err == nil {
		t.Fatal("expected error for media type person")
	}
}

func TestDiscoverYearFloor(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{YearGTE: 2024, Now: testNow}, 1)
	if got := q.Get("primary_release_date.gte"); got != "2024-01-01" {
		t.Fatalf("movie year floor: got %q", got)
	}

	_, q = discoverQueryFor(t, "tv", DiscoverFilters{YearGTE: 2024, Now: testNow}, 1)
	if got := q.Get("first_air_date.gte"); got != "2024-01-01" {
		t.Fatalf("tv year floor: got %q", got)
	}
	if q.Get("primary_release_date.gte") != "" {
		t.Fatal("tv must not send primary_release_date.gte")
	}
}

func TestDiscoverGenresAreAnded(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{GenreIDs: []int{80, 18}, Now: testNow}, 1)
	if got := q.Get("with_genres"); got != "18,80" {
		t.Fatalf("with_genres: got %q, want 18,80 (sorted ascending)", got)
	}
}

// Genre order is caller-controlled but semantically irrelevant to an AND
// filter, so it must not fork the cache key or the upstream query.
func TestDiscoverGenreOrderIsCanonicalised(t *testing.T) {
	s := newTestStore(t)
	var calls int
	var gotQueries []url.Values
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotQueries = append(gotQueries, r.URL.Query())
		w.Write([]byte(`{"page":1,"results":[]}`))
	}), s)

	orig := []int{80, 18}
	if _, err := c.Discover(context.Background(), "movie", DiscoverFilters{GenreIDs: orig, Now: testNow}, 1); err != nil {
		t.Fatalf("Discover([80,18]): %v", err)
	}
	if got := []int{80, 18}; orig[0] != got[0] || orig[1] != got[1] {
		t.Fatalf("Discover must not mutate the caller's GenreIDs slice, got %v", orig)
	}
	if _, err := c.Discover(context.Background(), "movie", DiscoverFilters{GenreIDs: []int{18, 80}, Now: testNow}, 1); err != nil {
		t.Fatalf("Discover([18,80]): %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 upstream call for equivalent genre sets, got %d", calls)
	}
	for _, q := range gotQueries {
		if got := q.Get("with_genres"); got != "18,80" {
			t.Fatalf("with_genres: got %q, want stable sorted order 18,80", got)
		}
	}
}

// Duplicate genre IDs are semantically a single filter and must not create a
// distinct cache key or upstream query from the single-ID case.
func TestDiscoverGenreDuplicatesAreDeduped(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{GenreIDs: []int{18, 18}, Now: testNow}, 1)
	if got := q.Get("with_genres"); got != "18" {
		t.Fatalf("with_genres: got %q, want 18 (deduped)", got)
	}
}

// A rating filter is client-supplied with arbitrary float precision; without
// quantisation each fractional variation would be a distinct, permanently
// cached upstream call.
func TestDiscoverRatingIsQuantised(t *testing.T) {
	s := newTestStore(t)
	var calls int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"page":1,"results":[]}`))
	}), s)

	if _, err := c.Discover(context.Background(), "movie", DiscoverFilters{RatingGTE: 7, Now: testNow}, 1); err != nil {
		t.Fatalf("Discover(rating=7): %v", err)
	}
	if _, err := c.Discover(context.Background(), "movie", DiscoverFilters{RatingGTE: 7.0000001, Now: testNow}, 1); err != nil {
		t.Fatalf("Discover(rating=7.0000001): %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 upstream call for equivalent ratings, got %d", calls)
	}
}

func TestDiscoverSortMapping(t *testing.T) {
	cases := []struct{ mediaType, sort, want string }{
		{"movie", "", "popularity.desc"},
		{"movie", "popularity", "popularity.desc"},
		{"movie", "rating", "vote_average.desc"},
		{"movie", "newest", "primary_release_date.desc"},
		{"tv", "newest", "first_air_date.desc"},
	}
	for _, tc := range cases {
		_, q := discoverQueryFor(t, tc.mediaType, DiscoverFilters{Sort: tc.sort, Now: testNow}, 1)
		if got := q.Get("sort_by"); got != tc.want {
			t.Fatalf("%s/%s: got sort_by %q, want %q", tc.mediaType, tc.sort, got, tc.want)
		}
	}
}

func TestDiscoverVoteCountGuard(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Now: testNow}, 1)
	if q.Get("vote_count.gte") != "" {
		t.Fatal("no guard expected without a rating filter or rating sort")
	}

	_, q = discoverQueryFor(t, "movie", DiscoverFilters{Sort: "rating", Now: testNow}, 1)
	if q.Get("vote_count.gte") != "100" {
		t.Fatalf("rating sort guard: got %q", q.Get("vote_count.gte"))
	}

	_, q = discoverQueryFor(t, "movie", DiscoverFilters{RatingGTE: 7, Now: testNow}, 1)
	if q.Get("vote_average.gte") != "7" {
		t.Fatalf("vote_average.gte: got %q", q.Get("vote_average.gte"))
	}
	if q.Get("vote_count.gte") != "100" {
		t.Fatalf("rating filter guard: got %q", q.Get("vote_count.gte"))
	}
}

// An unreleased title rarely has 100 votes yet, so the floor that guards
// rating filters elsewhere would empty out an upcoming page.
func TestDiscoverUpcomingSkipsVoteCountFloor(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Upcoming: true, RatingGTE: 7, Now: testNow}, 1)
	if q.Get("vote_count.gte") != "" {
		t.Fatalf("upcoming must not set a vote count floor, got %q", q.Get("vote_count.gte"))
	}
	if got := q.Get("vote_average.gte"); got != "7" {
		t.Fatalf("vote_average.gte: got %q, want 7", got)
	}
}

func TestDiscoverNonUpcomingKeepsVoteCountFloor(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Upcoming: false, RatingGTE: 7, Now: testNow}, 1)
	if got := q.Get("vote_count.gte"); got != "100" {
		t.Fatalf("non-upcoming rating guard: got %q, want 100", got)
	}
}

func TestDiscoverNewestCapsAtToday(t *testing.T) {
	_, q := discoverQueryFor(t, "tv", DiscoverFilters{Sort: "newest", Now: testNow}, 1)
	if got := q.Get("first_air_date.lte"); got != "2026-08-08" {
		t.Fatalf("newest ceiling: got %q, want 2026-08-08", got)
	}

	_, q = discoverQueryFor(t, "movie", DiscoverFilters{Sort: "newest", Now: testNow}, 1)
	if got := q.Get("primary_release_date.lte"); got != "2026-08-08" {
		t.Fatalf("movie newest ceiling: got %q", got)
	}
}

// Upcoming movies means "released from today"; upcoming TV means "has an episode
// airing from today", which is air_date, not the series premiere date. Both also
// carry an upper bound, mirroring the curated endpoints they replace: 3 months
// for movie.upcoming, 7 days for tv.on_the_air. The ceiling applies regardless of
// Sort so it is present here even though Sort is unset.
func TestDiscoverUpcomingUsesTheRightDateField(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Upcoming: true, Now: testNow}, 1)
	if got := q.Get("primary_release_date.gte"); got != "2026-08-08" {
		t.Fatalf("movie upcoming floor: got %q", got)
	}
	if got := q.Get("primary_release_date.lte"); got != "2026-11-08" {
		t.Fatalf("movie upcoming ceiling: got %q, want 2026-11-08", got)
	}

	_, q = discoverQueryFor(t, "tv", DiscoverFilters{Upcoming: true, Now: testNow}, 1)
	if got := q.Get("air_date.gte"); got != "2026-08-08" {
		t.Fatalf("tv upcoming floor: got %q, want air_date.gte=2026-08-08", got)
	}
	if got := q.Get("air_date.lte"); got != "2026-08-15" {
		t.Fatalf("tv upcoming ceiling: got %q, want 2026-08-15", got)
	}
	if q.Get("first_air_date.gte") != "" {
		t.Fatal("tv upcoming must not constrain first_air_date")
	}
}

func TestDiscoverUpcomingIgnoresYearAndNewestCeiling(t *testing.T) {
	_, q := discoverQueryFor(t, "tv", DiscoverFilters{Upcoming: true, YearGTE: 2000, Sort: "newest", Now: testNow}, 1)
	if q.Get("first_air_date.gte") != "" {
		t.Fatal("upcoming must ignore the year floor")
	}
	if q.Get("first_air_date.lte") != "" {
		t.Fatal("upcoming must not cap newest at today")
	}
}

// TMDB's /discover/tv has no region parameter; sending it would be a no-op that
// still varies the cache key.
func TestDiscoverRegionIsMovieOnly(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Region: "GB", Now: testNow}, 1)
	if q.Get("region") != "GB" {
		t.Fatalf("movie region: got %q", q.Get("region"))
	}

	_, q = discoverQueryFor(t, "tv", DiscoverFilters{Region: "GB", Now: testNow}, 1)
	if q.Get("region") != "" {
		t.Fatalf("tv must not send region, got %q", q.Get("region"))
	}
}

func TestDiscoverPassesPage(t *testing.T) {
	_, q := discoverQueryFor(t, "movie", DiscoverFilters{Now: testNow}, 3)
	if q.Get("page") != "3" {
		t.Fatalf("page: got %q", q.Get("page"))
	}
}

func TestDiscoverCacheKeyVariesByEveryInput(t *testing.T) {
	base := DiscoverFilters{YearGTE: 2024, GenreIDs: []int{80}, Sort: "rating", RatingGTE: 7, Region: "US", Now: testNow}

	cases := []struct {
		name      string
		mediaType string
		filters   DiscoverFilters
		page      int
	}{
		{"year", "movie", func() DiscoverFilters { f := base; f.YearGTE = 2020; return f }(), 1},
		{"genres", "movie", func() DiscoverFilters { f := base; f.GenreIDs = []int{18}; return f }(), 1},
		{"sort", "movie", func() DiscoverFilters { f := base; f.Sort = "newest"; return f }(), 1},
		{"rating", "movie", func() DiscoverFilters { f := base; f.RatingGTE = 8; return f }(), 1},
		{"region", "movie", func() DiscoverFilters { f := base; f.Region = "GB"; return f }(), 1},
		{"upcoming", "movie", func() DiscoverFilters { f := base; f.Upcoming = true; return f }(), 1},
		{"page", "movie", base, 2},
		{"mediaType", "tv", base, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Write([]byte(`{"page":1,"results":[]}`))
			}), newTestStore(t))
			ctx := context.Background()

			if _, err := c.Discover(ctx, "movie", base, 1); err != nil {
				t.Fatalf("baseline: %v", err)
			}
			if _, err := c.Discover(ctx, "movie", base, 1); err != nil {
				t.Fatalf("baseline repeat: %v", err)
			}
			if calls != 1 {
				t.Fatalf("identical inputs must hit the cache, got %d upstream calls", calls)
			}

			if _, err := c.Discover(ctx, tc.mediaType, tc.filters, tc.page); err != nil {
				t.Fatalf("variant: %v", err)
			}
			if calls != 2 {
				t.Fatalf("changing %s must miss the cache, got %d upstream calls", tc.name, calls)
			}
		})
	}
}

func TestGetGenres(t *testing.T) {
	cases := []struct {
		mediaType, wantPath string
	}{
		{"movie", "/genre/movie/list"},
		{"tv", "/genre/tv/list"},
	}
	for _, tc := range cases {
		var gotPath string
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(`{"genres":[{"id":80,"name":"Crime"}]}`))
		}), newTestStore(t))

		data, err := c.GetGenres(context.Background(), tc.mediaType)
		if err != nil {
			t.Fatalf("GetGenres(%s): %v", tc.mediaType, err)
		}
		if gotPath != tc.wantPath {
			t.Fatalf("%s: path: got %s, want %s", tc.mediaType, gotPath, tc.wantPath)
		}
		if !strings.Contains(string(data), "Crime") {
			t.Fatalf("%s: body: got %s", tc.mediaType, data)
		}
	}
}
