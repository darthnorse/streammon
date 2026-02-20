package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestTMDBNoClient(t *testing.T) {
	srv, _ := newTestServerWrapped(t)
	// tmdbClient is nil by default

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no TMDB client, got %d", w.Code)
	}
}
