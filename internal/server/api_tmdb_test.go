package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestTMDBNoClient(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no TMDB client, got %d", w.Code)
	}
}
