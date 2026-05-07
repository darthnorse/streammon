package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
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
	if err := st.ExecForTest(context.Background(),
		`INSERT INTO library_items
			(server_id, library_id, item_id, media_type, title, year, added_at, episode_count)
		 VALUES
			(1,'l','a','movie','A',2020,'2024-01-01',0),
			(1,'l','b','movie','B',2021,'2024-01-02',0),
			(1,'l','c','episode','Show',2019,'2024-01-03',3),
			(2,'l','d','movie','D',2022,'2024-01-04',0),
			(2,'l','e','episode','Show2',2018,'2024-01-05',10)`,
	); err != nil {
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

	// Totals: 5 rows (3 + 2), 3 movies, 2 series, 13 episodes total.
	if resp.TotalItems != 5 || resp.Movies != 3 || resp.Shows != 2 || resp.Episodes != 13 {
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
