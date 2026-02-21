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

func (m *mockItemDetailsServer) Name() string                  { return "mock" }
func (m *mockItemDetailsServer) Type() models.ServerType       { return models.ServerTypePlex }
func (m *mockItemDetailsServer) ServerID() int64               { return m.serverID }
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

