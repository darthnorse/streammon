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
	srv, _ := newTestServer(t)

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
	if resp.ConcurrentPeak != 0 {
		t.Fatalf("expected 0 concurrent peak, got %d", resp.ConcurrentPeak)
	}
	if len(resp.Locations) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(resp.Locations))
	}
	if len(resp.PotentialSharers) != 0 {
		t.Fatalf("expected 0 potential sharers, got %d", len(resp.PotentialSharers))
	}
}

func TestGetStatsAPI_WithData(t *testing.T) {
	srv, st := newTestServer(t)

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

	if resp.ConcurrentPeak < 2 {
		t.Fatalf("expected concurrent peak >= 2, got %d", resp.ConcurrentPeak)
	}
}
