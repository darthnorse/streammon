package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestGetStatsAPI_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp StatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.TopMovies) != 0 {
		t.Fatalf("expected 0 top movies, got %d", len(resp.TopMovies))
	}
	if len(resp.TopTVShows) != 0 {
		t.Fatalf("expected 0 top tv shows, got %d", len(resp.TopTVShows))
	}
	if len(resp.TopUsers) != 0 {
		t.Fatalf("expected 0 top users, got %d", len(resp.TopUsers))
	}
	if resp.Library == nil {
		t.Fatal("expected library stats")
	}
	if resp.Library.TotalPlays != 0 {
		t.Fatalf("expected 0 total plays, got %d", resp.Library.TotalPlays)
	}
	if resp.ConcurrentPeaks.Total != 0 {
		t.Fatalf("expected 0 concurrent peak, got %d", resp.ConcurrentPeaks.Total)
	}
	if len(resp.Locations) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(resp.Locations))
	}
}

func TestGetStatsAPI_WithData(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	st.CreateServer(s)

	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", Year: 1999, WatchedMs: 7200000,
		StartedAt: now.Add(-2 * time.Hour), StoppedAt: now,
	})
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", Year: 1999, WatchedMs: 7200000,
		StartedAt: now.Add(-1 * time.Hour), StoppedAt: now.Add(time.Hour),
	})
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "S01E01", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now.Add(-30 * time.Minute), StoppedAt: now.Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp StatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.TopMovies) != 1 {
		t.Fatalf("expected 1 top movie, got %d", len(resp.TopMovies))
	}
	if resp.TopMovies[0].Title != "The Matrix" {
		t.Fatalf("expected The Matrix, got %s", resp.TopMovies[0].Title)
	}
	if resp.TopMovies[0].PlayCount != 2 {
		t.Fatalf("expected 2 plays, got %d", resp.TopMovies[0].PlayCount)
	}

	if len(resp.TopTVShows) != 1 {
		t.Fatalf("expected 1 top tv show, got %d", len(resp.TopTVShows))
	}
	if resp.TopTVShows[0].Title != "Breaking Bad" {
		t.Fatalf("expected Breaking Bad, got %s", resp.TopTVShows[0].Title)
	}

	if len(resp.TopUsers) != 2 {
		t.Fatalf("expected 2 top users, got %d", len(resp.TopUsers))
	}

	if resp.Library.TotalPlays != 3 {
		t.Fatalf("expected 3 total plays, got %d", resp.Library.TotalPlays)
	}
	if resp.Library.UniqueUsers != 2 {
		t.Fatalf("expected 2 unique users, got %d", resp.Library.UniqueUsers)
	}

	if resp.ConcurrentPeaks.Total < 2 {
		t.Fatalf("expected concurrent peak >= 2, got %d", resp.ConcurrentPeaks.Total)
	}
}

func TestGetStatsAPI_WithLocations(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	st.CreateServer(s)

	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie", IPAddress: "8.8.8.8",
		StartedAt: now.Add(-time.Hour), StoppedAt: now,
	})

	st.SetCachedGeo(&models.GeoResult{
		IP: "8.8.8.8", City: "Mountain View", Country: "US", Lat: 37.386, Lng: -122.084,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp StatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(resp.Locations))
	}
	if resp.Locations[0].City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", resp.Locations[0].City)
	}
}

func TestGetStatsAPI_WithDaysFilter(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"days=7", "?days=7", http.StatusOK},
		{"days=30", "?days=30", http.StatusOK},
		{"days=0", "?days=0", http.StatusOK},
		{"days=14", "?days=14", http.StatusOK},
		{"invalid days", "?days=invalid", http.StatusBadRequest},
		{"negative days", "?days=-1", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/stats"+tt.query, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetStatsAPI_WithDateRange(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest("GET", "/api/stats?start_date=2024-01-01&end_date=2024-02-01", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestGetStatsAPI_InvalidDateRange(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	tests := []struct {
		name  string
		query string
	}{
		{"bad start_date", "?start_date=invalid"},
		{"bad end_date", "?start_date=2024-01-01&end_date=invalid"},
		{"end before start", "?start_date=2024-03-01&end_date=2024-01-01"},
		{"start_date only", "?start_date=2024-01-01"},
		{"end_date only", "?end_date=2024-02-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/stats"+tt.query, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestGetStatsAPI_WithServerIDs(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest("GET", "/api/stats?server_ids=1,2", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestGetStatsAPI_InvalidServerIDs(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest("GET", "/api/stats?server_ids=abc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
