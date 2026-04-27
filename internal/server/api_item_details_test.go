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

func setupItemDetailsHistory(t *testing.T) *store.Store {
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
	return st
}

func TestItemDetailsHistoryAdminSeesAllUsers(t *testing.T) {
	st := setupItemDetailsHistory(t)
	history, err := st.HistoryForTitleByUser("Test Movie", "", 10)
	if err != nil {
		t.Fatalf("HistoryForTitleByUser: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries for admin (all users), got %d", len(history))
	}
}

func TestItemDetailsHistoryViewerSeesOnlyOwnHistory(t *testing.T) {
	st := setupItemDetailsHistory(t)
	history, err := st.HistoryForTitleByUser("Test Movie", "alice", 10)
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
