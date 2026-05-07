package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
	"streammon/internal/store"
)

func TestLibrarySummary_TotalsAcrossServers(t *testing.T) {
	ts, st := newTestServerWrapped(t)

	if err := st.CreateServer(&models.Server{Name: "Plex", Type: "plex", URL: "http://x"}); err != nil {
		t.Fatalf("create server: %v", err)
	}
	if err := st.CreateServer(&models.Server{Name: "Emby", Type: "emby", URL: "http://y"}); err != nil {
		t.Fatalf("create server: %v", err)
	}

	// Server 1: 2 movies + 1 series with episode_count=3
	// Server 2: 1 movie + 1 series with episode_count=10
	if err := st.SeedLibraryItemsForTest(context.Background(), []store.LibraryItemSeed{
		{ServerID: 1, LibraryID: "l", ItemID: "a", MediaType: "movie", Title: "A", Year: 2020, AddedAt: "2024-01-01"},
		{ServerID: 1, LibraryID: "l", ItemID: "b", MediaType: "movie", Title: "B", Year: 2021, AddedAt: "2024-01-02"},
		{ServerID: 1, LibraryID: "l", ItemID: "c", MediaType: "episode", Title: "Show", Year: 2019, AddedAt: "2024-01-03", EpisodeCount: 3},
		{ServerID: 2, LibraryID: "l", ItemID: "d", MediaType: "movie", Title: "D", Year: 2022, AddedAt: "2024-01-04"},
		{ServerID: 2, LibraryID: "l", ItemID: "e", MediaType: "episode", Title: "Show2", Year: 2018, AddedAt: "2024-01-05", EpisodeCount: 10},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/library/summary", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp librarySummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Totals: 5 rows (3 + 2), 3 movies, 2 series, 13 episodes total, 0 other.
	if resp.TotalItems != 5 || resp.Movies != 3 || resp.Shows != 2 || resp.Episodes != 13 || resp.Other != 0 {
		t.Errorf("totals wrong: %+v", resp)
	}
	if len(resp.PerServer) != 2 {
		t.Fatalf("expected 2 per-server entries, got %d", len(resp.PerServer))
	}
	names := map[string]bool{}
	for _, p := range resp.PerServer {
		names[p.ServerName] = true
	}
	if !names["Plex"] || !names["Emby"] {
		t.Errorf("expected server names in per_server, got %+v", resp.PerServer)
	}
}

func TestLibrarySummary_EmptyReturnsZeros(t *testing.T) {
	ts, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/summary", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var resp librarySummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalItems != 0 || len(resp.PerServer) != 0 {
		t.Errorf("expected zero totals + empty list, got %+v", resp)
	}
}
