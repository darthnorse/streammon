package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestListHistoryAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result models.PaginatedResult[models.WatchHistoryEntry]
	json.NewDecoder(w.Body).Decode(&result)
	if result.Total != 0 {
		t.Fatalf("expected 0 total, got %d", result.Total)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}

	s := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	st.CreateServer(s)
	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Test", StartedAt: now, StoppedAt: now,
	})

	req = httptest.NewRequest(http.MethodGet, "/api/history?page=1&per_page=10", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&result)
	if result.Total != 1 {
		t.Fatalf("expected 1 total, got %d", result.Total)
	}
}

func TestListHistoryWithFilterAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	s := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	st.CreateServer(s)
	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "A", StartedAt: now, StoppedAt: now,
	})
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "B", StartedAt: now, StoppedAt: now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/history?user=alice", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var result models.PaginatedResult[models.WatchHistoryEntry]
	json.NewDecoder(w.Body).Decode(&result)
	if result.Total != 1 {
		t.Fatalf("expected 1, got %d", result.Total)
	}
}

func TestDailyHistoryAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history/daily?start=2024-06-01&end=2024-06-03", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats []models.DayStat
	json.NewDecoder(w.Body).Decode(&stats)
	if len(stats) != 0 {
		t.Fatalf("expected 0 stats, got %d", len(stats))
	}
}

func TestDailyHistoryBadDatesAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	tests := []struct {
		name string
		url  string
	}{
		{"missing start", "/api/history/daily?end=2024-06-03"},
		{"missing end", "/api/history/daily?start=2024-06-01"},
		{"bad start", "/api/history/daily?start=nope&end=2024-06-03"},
		{"bad end", "/api/history/daily?start=2024-06-01&end=nope"},
		{"end before start", "/api/history/daily?start=2024-06-03&end=2024-06-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestListHistoryPerPageCapAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history?per_page=9999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var result models.PaginatedResult[models.WatchHistoryEntry]
	json.NewDecoder(w.Body).Decode(&result)
	if result.PerPage != 100 {
		t.Fatalf("expected per_page capped to 100, got %d", result.PerPage)
	}
}
