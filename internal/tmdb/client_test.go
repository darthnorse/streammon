package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/time/rate"

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

	data, err := c.Trending(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("got %s, want %s", data, expected)
	}
}
