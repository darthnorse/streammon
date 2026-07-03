package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

func setupItemDetailsHistory(t *testing.T) (*store.Store, int64) {
	t.Helper()
	_, st := newTestServer(t)
	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	now := time.Now().UTC()
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Test Movie", StartedAt: now, StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Test Movie", StartedAt: now.Add(-time.Hour), StoppedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	return st, s.ID
}

func TestItemDetailsHistoryAdminSeesAllUsers(t *testing.T) {
	st, serverID := setupItemDetailsHistory(t)
	history, err := st.HistoryForTitleByUser(serverID, "Test Movie", "", 10)
	if err != nil {
		t.Fatalf("HistoryForTitleByUser: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries for admin (all users), got %d", len(history))
	}
}

func TestItemDetailsHistoryViewerSeesOnlyOwnHistory(t *testing.T) {
	st, serverID := setupItemDetailsHistory(t)
	history, err := st.HistoryForTitleByUser(serverID, "Test Movie", "alice", 10)
	if err != nil {
		t.Fatalf("HistoryForTitleByUser: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry for alice only, got %d", len(history))
	}
	if history[0].UserName != "alice" {
		t.Fatalf("expected alice's history, got %s", history[0].UserName)
	}
}

type mockItemDetailsServer struct {
	serverID int64
	details  *models.ItemDetails
	err      error
}

func (m *mockItemDetailsServer) Name() string            { return "mock" }
func (m *mockItemDetailsServer) Type() models.ServerType { return models.ServerTypePlex }
func (m *mockItemDetailsServer) ServerID() int64         { return m.serverID }
func (m *mockItemDetailsServer) TestConnection(ctx context.Context) error {
	return nil
}
func (m *mockItemDetailsServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) DeleteItem(ctx context.Context, itemID string) error {
	return nil
}
func (m *mockItemDetailsServer) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) GetEpisodes(ctx context.Context, seasonID string) ([]models.Episode, error) {
	return nil, nil
}
func (m *mockItemDetailsServer) TerminateSession(ctx context.Context, sessionID string, message string) error {
	return nil
}
func (m *mockItemDetailsServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.details, nil
}

// TestItemDetailsNilPoller guards against a nil-pointer panic when the
// server has no poller configured (e.g. no media servers set up yet).
func TestItemDetailsNilPoller(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/1/items/item-1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestItemDetailsIncludesTMDBID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: s.ID, LibraryID: "lib1", ItemID: "item-42", MediaType: models.MediaTypeMovie, Title: "Test Movie", TMDBID: "550", AddedAt: now, SyncedAt: now},
	}
	if _, err := st.UpsertLibraryItems(context.Background(), items); err != nil {
		t.Fatalf("UpsertLibraryItems: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockItemDetailsServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID:         "item-42",
			Title:      "Test Movie",
			MediaType:  models.MediaTypeMovie,
			ServerID:   s.ID,
			ServerName: "Plex",
			ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/item-42", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TMDBID != "550" {
		t.Fatalf("expected tmdb_id=550, got %q", resp.TMDBID)
	}
}

func TestItemDetailsNoTMDBID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockItemDetailsServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID:         "item-99",
			Title:      "No TMDB",
			MediaType:  models.MediaTypeMovie,
			ServerID:   s.ID,
			ServerName: "Plex",
			ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/item-99", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TMDBID != "" {
		t.Fatalf("expected empty tmdb_id, got %q", resp.TMDBID)
	}
}

func TestItemDetailsEpisodeFiltersByItemID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	now := time.Now().UTC().Add(-12 * time.Hour)

	// Two distinct episodes of the same show. Same grandparent, different item_id.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, ItemID: "ep-1", GrandparentItemID: "show-1",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Pilot", GrandparentTitle: "My Show",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now, StoppedAt: now.Add(30 * time.Minute),
	}); err != nil {
		t.Fatalf("InsertHistory ep1: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, ItemID: "ep-2", GrandparentItemID: "show-1",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Second", GrandparentTitle: "My Show",
		SeasonNumber: 1, EpisodeNumber: 2,
		StartedAt: now.Add(2 * time.Hour), StoppedAt: now.Add(2*time.Hour + 30*time.Minute),
	}); err != nil {
		t.Fatalf("InsertHistory ep2: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockItemDetailsServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID:            "ep-1",
			Title:         "Pilot",
			MediaType:     models.MediaTypeTV,
			Level:         "episode",
			SeriesID:      "show-1",
			ParentID:      "season-1",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			SeriesTitle:   "My Show",
			ServerID:      s.ID,
			ServerName:    "Plex",
			ServerType:    models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/ep-1", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 1 {
		t.Fatalf("expected 1 history entry for ep-1, got %d", len(resp.WatchHistory))
	}
	if resp.WatchHistory[0].ItemID != "ep-1" {
		t.Fatalf("expected ep-1, got %q", resp.WatchHistory[0].ItemID)
	}
}

// Old rows missing grandparent_item_id should still surface via the title-match
// fallback at show level; rows on a different server must not leak through.
func TestItemDetailsShowFallbackIsServerScoped(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	plex := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "p1", Enabled: true}
	emby := &models.Server{Name: "Emby", Type: models.ServerTypeEmby, URL: "http://emby", APIKey: "k", MachineID: "e1", Enabled: true}
	if err := st.CreateServer(plex); err != nil {
		t.Fatalf("CreateServer plex: %v", err)
	}
	if err := st.CreateServer(emby); err != nil {
		t.Fatalf("CreateServer emby: %v", err)
	}

	now := time.Now().UTC().Add(-12 * time.Hour)

	// Old row on Plex without grandparent_item_id (forces fallback).
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: plex.ID, ItemID: "old-ep", GrandparentItemID: "",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Pilot", GrandparentTitle: "Common Title",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now, StoppedAt: now.Add(20 * time.Minute),
	}); err != nil {
		t.Fatalf("insert plex: %v", err)
	}
	// Same-titled row on Emby — must NOT leak when querying Plex.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: emby.ID, ItemID: "emby-ep", GrandparentItemID: "",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Pilot", GrandparentTitle: "Common Title",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now.Add(time.Hour), StoppedAt: now.Add(time.Hour + 20*time.Minute),
	}); err != nil {
		t.Fatalf("insert emby: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(plex.ID, &mockItemDetailsServer{
		serverID: plex.ID,
		details: &models.ItemDetails{
			ID: "show-Y", Title: "Common Title", MediaType: models.MediaTypeTV,
			Level: "show", SeriesTitle: "Common Title",
			ServerID: plex.ID, ServerName: "Plex", ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/show-Y", plex.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 1 {
		t.Fatalf("expected 1 history entry from Plex only (fallback must be server-scoped), got %d", len(resp.WatchHistory))
	}
	if resp.WatchHistory[0].ServerID != plex.ID {
		t.Fatalf("fallback returned cross-server row: server_id=%d", resp.WatchHistory[0].ServerID)
	}
}

func TestItemDetailsConsolidatesAcrossServersByTMDBID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s1 := &models.Server{Name: "Plex 4K", Type: models.ServerTypePlex, URL: "http://plex1", APIKey: "k1", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s1); err != nil {
		t.Fatalf("CreateServer s1: %v", err)
	}
	s2 := &models.Server{Name: "Plex HD", Type: models.ServerTypePlex, URL: "http://plex2", APIKey: "k2", MachineID: "m2", Enabled: true}
	if err := st.CreateServer(s2); err != nil {
		t.Fatalf("CreateServer s2: %v", err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: s1.ID, LibraryID: "lib1", ItemID: "movie-4k", MediaType: models.MediaTypeMovie, Title: "Inception", TMDBID: "550", AddedAt: now, SyncedAt: now},
		{ServerID: s2.ID, LibraryID: "lib2", ItemID: "movie-hd", MediaType: models.MediaTypeMovie, Title: "Inception", TMDBID: "550", AddedAt: now, SyncedAt: now},
	}
	if _, err := st.UpsertLibraryItems(context.Background(), items); err != nil {
		t.Fatalf("UpsertLibraryItems: %v", err)
	}

	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s1.ID, ItemID: "movie-4k", UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Inception", StartedAt: now.Add(-2 * time.Hour), StoppedAt: now.Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("InsertHistory s1: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s2.ID, ItemID: "movie-hd", UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Inception", StartedAt: now.Add(-30 * time.Minute), StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory s2: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s1.ID, &mockItemDetailsServer{
		serverID: s1.ID,
		details: &models.ItemDetails{
			ID:         "movie-4k",
			Title:      "Inception",
			MediaType:  models.MediaTypeMovie,
			Level:      "movie",
			ServerID:   s1.ID,
			ServerName: "Plex 4K",
			ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/movie-4k", s1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 2 {
		t.Fatalf("expected 2 merged history rows (alice on s1 + bob on s2), got %d", len(resp.WatchHistory))
	}
	users := map[string]bool{resp.WatchHistory[0].UserName: true, resp.WatchHistory[1].UserName: true}
	if !users["alice"] || !users["bob"] {
		t.Fatalf("expected alice+bob, got %v", users)
	}
}

func TestItemDetailsNoTMDBIDStaysSingleServer(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s1 := &models.Server{Name: "Plex 1", Type: models.ServerTypePlex, URL: "http://plex1", APIKey: "k1", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s1); err != nil {
		t.Fatalf("CreateServer s1: %v", err)
	}
	s2 := &models.Server{Name: "Plex 2", Type: models.ServerTypePlex, URL: "http://plex2", APIKey: "k2", MachineID: "m2", Enabled: true}
	if err := st.CreateServer(s2); err != nil {
		t.Fatalf("CreateServer s2: %v", err)
	}

	now := time.Now().UTC()
	// Two history rows on different servers, but no library_items entries — no tmdb_id to resolve.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s1.ID, ItemID: "home-movie", UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Holiday 2024", StartedAt: now, StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory s1: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s2.ID, ItemID: "home-movie-other", UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Holiday 2024", StartedAt: now, StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory s2: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s1.ID, &mockItemDetailsServer{
		serverID: s1.ID,
		details: &models.ItemDetails{
			ID:         "home-movie",
			Title:      "Holiday 2024",
			MediaType:  models.MediaTypeMovie,
			Level:      "movie",
			ServerID:   s1.ID,
			ServerName: "Plex 1",
			ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/home-movie", s1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 1 {
		t.Fatalf("expected 1 history row (single-server, no tmdb), got %d", len(resp.WatchHistory))
	}
	if resp.WatchHistory[0].UserName != "alice" {
		t.Fatalf("expected alice (s1), got %s", resp.WatchHistory[0].UserName)
	}
}

func TestItemDetailsConsolidatesEpisodeAcrossServersByTMDBID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s1 := &models.Server{Name: "Plex 4K", Type: models.ServerTypePlex, URL: "http://plex1", APIKey: "k1", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s1); err != nil {
		t.Fatalf("CreateServer s1: %v", err)
	}
	s2 := &models.Server{Name: "Plex HD", Type: models.ServerTypePlex, URL: "http://plex2", APIKey: "k2", MachineID: "m2", Enabled: true}
	if err := st.CreateServer(s2); err != nil {
		t.Fatalf("CreateServer s2: %v", err)
	}

	now := time.Now().UTC()
	// Same show on both servers, each with its own item_id, sharing a tmdb_id.
	showItems := []models.LibraryItemCache{
		{ServerID: s1.ID, LibraryID: "lib1", ItemID: "show-on-s1", MediaType: models.MediaTypeTV, Title: "Breaking Bad", TMDBID: "1100", AddedAt: now, SyncedAt: now},
		{ServerID: s2.ID, LibraryID: "lib2", ItemID: "show-on-s2", MediaType: models.MediaTypeTV, Title: "Breaking Bad", TMDBID: "1100", AddedAt: now, SyncedAt: now},
	}
	if _, err := st.UpsertLibraryItems(context.Background(), showItems); err != nil {
		t.Fatalf("UpsertLibraryItems: %v", err)
	}

	// One episode watch_history per server; grandparent_item_id matches that server's show item_id.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s1.ID, ItemID: "ep-s1-1-1", GrandparentItemID: "show-on-s1",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Pilot", GrandparentTitle: "Breaking Bad",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now.Add(-2 * time.Hour), StoppedAt: now.Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("InsertHistory s1: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s2.ID, ItemID: "ep-s2-1-1", GrandparentItemID: "show-on-s2",
		UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "Pilot", GrandparentTitle: "Breaking Bad",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now.Add(-30 * time.Minute), StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory s2: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	// Mock represents the episode opened from s1.
	p.AddServer(s1.ID, &mockItemDetailsServer{
		serverID: s1.ID,
		details: &models.ItemDetails{
			ID:            "ep-s1-1-1",
			Title:         "Pilot",
			MediaType:     models.MediaTypeTV,
			Level:         "episode",
			SeriesID:      "show-on-s1",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			SeriesTitle:   "Breaking Bad",
			ServerID:      s1.ID,
			ServerName:    "Plex 4K",
			ServerType:    models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/ep-s1-1-1", s1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 2 {
		t.Fatalf("expected 2 merged history rows (alice on s1 + bob on s2), got %d", len(resp.WatchHistory))
	}
	users := map[string]bool{resp.WatchHistory[0].UserName: true, resp.WatchHistory[1].UserName: true}
	if !users["alice"] || !users["bob"] {
		t.Fatalf("expected alice+bob, got %v", users)
	}
}

// TestItemDetailsFallsBackWhenCrossServerEmpty verifies that when library_items
// has been re-synced with new item IDs but watch_history still references the
// old IDs, the cross-server query returns zero rows and the single-server fallback
// path fires instead.
func TestItemDetailsFallsBackWhenCrossServerEmpty(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s1 := &models.Server{Name: "Plex 4K", Type: models.ServerTypePlex, URL: "http://plex1", APIKey: "k1", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s1); err != nil {
		t.Fatalf("CreateServer s1: %v", err)
	}
	s2 := &models.Server{Name: "Plex HD", Type: models.ServerTypePlex, URL: "http://plex2", APIKey: "k2", MachineID: "m2", Enabled: true}
	if err := st.CreateServer(s2); err != nil {
		t.Fatalf("CreateServer s2: %v", err)
	}

	now := time.Now().UTC()
	// library_items has been re-synced: item IDs are "new-id-*".
	items := []models.LibraryItemCache{
		{ServerID: s1.ID, LibraryID: "lib1", ItemID: "new-id-1", MediaType: models.MediaTypeMovie, Title: "Fight Club", TMDBID: "550", AddedAt: now, SyncedAt: now},
		{ServerID: s2.ID, LibraryID: "lib2", ItemID: "new-id-2", MediaType: models.MediaTypeMovie, Title: "Fight Club", TMDBID: "550", AddedAt: now, SyncedAt: now},
	}
	if _, err := st.UpsertLibraryItems(context.Background(), items); err != nil {
		t.Fatalf("UpsertLibraryItems: %v", err)
	}

	// watch_history still uses the old item ID "old-id-1" from before the re-sync.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s1.ID, ItemID: "old-id-1", UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Fight Club", StartedAt: now.Add(-time.Hour), StoppedAt: now,
	}); err != nil {
		t.Fatalf("InsertHistory s1: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	// The handler is invoked with "old-id-1" — the item the user clicked from s1.
	// GetLibraryItemTMDBID will look up server=s1, item="old-id-1" in library_items
	// and find nothing (that row now has item_id "new-id-1"), so tmdbID stays empty.
	// The cross-server path is skipped entirely; HistoryForItem falls back using
	// the original itemID "old-id-1" and returns alice's row.
	p.AddServer(s1.ID, &mockItemDetailsServer{
		serverID: s1.ID,
		details: &models.ItemDetails{
			ID:         "old-id-1",
			Title:      "Fight Club",
			MediaType:  models.MediaTypeMovie,
			Level:      "movie",
			ServerID:   s1.ID,
			ServerName: "Plex 4K",
			ServerType: models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/old-id-1", s1.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 1 {
		t.Fatalf("expected 1 history row (single-server fallback for stale item_id), got %d", len(resp.WatchHistory))
	}
	if resp.WatchHistory[0].UserName != "alice" {
		t.Fatalf("expected alice, got %s", resp.WatchHistory[0].UserName)
	}
}

func TestItemDetailsShowFiltersByGrandparent(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	now := time.Now().UTC().Add(-12 * time.Hour)

	// Two episodes of the show; both should match at show level.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, ItemID: "ep-A", GrandparentItemID: "show-X",
		UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "A", GrandparentTitle: "Show X",
		SeasonNumber: 1, EpisodeNumber: 1,
		StartedAt: now, StoppedAt: now.Add(20 * time.Minute),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: s.ID, ItemID: "ep-B", GrandparentItemID: "show-X",
		UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "B", GrandparentTitle: "Show X",
		SeasonNumber: 1, EpisodeNumber: 2,
		StartedAt: now.Add(time.Hour), StoppedAt: now.Add(time.Hour + 20*time.Minute),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockItemDetailsServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID:          "show-X",
			Title:       "Show X",
			MediaType:   models.MediaTypeTV,
			Level:       "show",
			SeriesTitle: "Show X",
			ServerID:    s.ID,
			ServerName:  "Plex",
			ServerType:  models.ServerTypePlex,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/items/show-X", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp itemDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.WatchHistory) != 2 {
		t.Fatalf("expected 2 history entries for show-X, got %d", len(resp.WatchHistory))
	}
}
