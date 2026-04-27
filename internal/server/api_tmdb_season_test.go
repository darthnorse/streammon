package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTMDBSeason(t *testing.T) {
	t.Run("success returns payload", func(t *testing.T) {
		expected := `{"name":"Season 1","air_date":"2010-01-01","episodes":[{"episode_number":1,"name":"Pilot"}]}`
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expected))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/tv/123/season/1", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != expected {
			t.Fatalf("got %s, want %s", w.Body.String(), expected)
		}
	})

	t.Run("invalid tmdb id returns 400", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/tv/abc/season/1", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid season returns 400", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/tv/123/season/foo", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("upstream 404 returns bad gateway", func(t *testing.T) {
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/tv/999/season/1", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("cache hit on second call", func(t *testing.T) {
		var calls int32
		expected := `{"name":"Season 1"}`
		srv, _ := newTestServerWithTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/tv/77/season/2") {
				t.Errorf("unexpected upstream path: %s", r.URL.Path)
			}
			atomic.AddInt32(&calls, 1)
			w.Write([]byte(expected))
		}))

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/tmdb/tv/77/season/2", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("call %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
			}
		}

		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("expected upstream called once, got %d", got)
		}
	})
}
