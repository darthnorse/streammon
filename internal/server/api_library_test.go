package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
)

type mockLibraryServer struct {
	name      string
	srvType   models.ServerType
	libraries []models.Library
	err       error
}

func (m *mockLibraryServer) Name() string                                            { return m.name }
func (m *mockLibraryServer) Type() models.ServerType                                 { return m.srvType }
func (m *mockLibraryServer) TestConnection(ctx context.Context) error                { return nil }
func (m *mockLibraryServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockLibraryServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockLibraryServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	return nil, nil
}
func (m *mockLibraryServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.libraries, nil
}

func setupTestPoller(t *testing.T, srv *Server, st *store.Store) *poller.Poller {
	t.Helper()
	p := poller.New(st, time.Hour)
	srv.poller = p

	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	return p
}

func TestGetLibrariesWithoutPoller(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.InvalidateLibraryCache()

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LibrariesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Libraries) != 0 {
		t.Fatalf("expected 0 libraries without poller, got %d", len(resp.Libraries))
	}
}

func TestGetLibrariesWithPoller(t *testing.T) {
	srv, st := newTestServer(t)
	srv.InvalidateLibraryCache()

	s := &models.Server{
		Name:    "TestPlex",
		Type:    models.ServerTypePlex,
		URL:     "http://plex",
		APIKey:  "k1",
		Enabled: true,
	}
	st.CreateServer(s)

	p := setupTestPoller(t, srv, st)

	mockServer := &mockLibraryServer{
		name:    "TestPlex",
		srvType: models.ServerTypePlex,
		libraries: []models.Library{
			{ID: "1", ServerID: s.ID, ServerName: "TestPlex", ServerType: models.ServerTypePlex, Name: "Movies", Type: models.LibraryTypeMovie, ItemCount: 100},
			{ID: "2", ServerID: s.ID, ServerName: "TestPlex", ServerType: models.ServerTypePlex, Name: "TV Shows", Type: models.LibraryTypeShow, ItemCount: 50, ChildCount: 200, GrandchildCount: 2500},
		},
	}
	p.AddServer(s.ID, mockServer)

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LibrariesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Libraries) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(resp.Libraries))
	}

	if resp.Libraries[0].Name != "Movies" {
		t.Errorf("first library name = %q, want Movies", resp.Libraries[0].Name)
	}
	if resp.Libraries[0].ItemCount != 100 {
		t.Errorf("movies item_count = %d, want 100", resp.Libraries[0].ItemCount)
	}
	if resp.Libraries[1].GrandchildCount != 2500 {
		t.Errorf("tv grandchild_count = %d, want 2500", resp.Libraries[1].GrandchildCount)
	}
}

func TestGetLibrariesMultipleServers(t *testing.T) {
	srv, st := newTestServer(t)
	srv.InvalidateLibraryCache()

	s1 := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k1", Enabled: true}
	st.CreateServer(s1)
	s2 := &models.Server{Name: "Emby", Type: models.ServerTypeEmby, URL: "http://emby", APIKey: "k2", Enabled: true}
	st.CreateServer(s2)

	p := setupTestPoller(t, srv, st)

	p.AddServer(s1.ID, &mockLibraryServer{
		name:      "Plex",
		srvType:   models.ServerTypePlex,
		libraries: []models.Library{{ID: "1", Name: "Plex Movies", ServerID: s1.ID}},
	})
	p.AddServer(s2.ID, &mockLibraryServer{
		name:      "Emby",
		srvType:   models.ServerTypeEmby,
		libraries: []models.Library{{ID: "1", Name: "Emby Movies", ServerID: s2.ID}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LibrariesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Libraries) != 2 {
		t.Fatalf("expected 2 libraries from 2 servers, got %d", len(resp.Libraries))
	}
}

func TestGetLibrariesServerError(t *testing.T) {
	srv, st := newTestServer(t)
	srv.InvalidateLibraryCache()

	s := &models.Server{Name: "FailingPlex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k1", Enabled: true}
	st.CreateServer(s)

	p := setupTestPoller(t, srv, st)

	p.AddServer(s.ID, &mockLibraryServer{
		name:    "FailingPlex",
		srvType: models.ServerTypePlex,
		err:     errors.New("connection refused"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even with error, got %d: %s", w.Code, w.Body.String())
	}

	var resp LibrariesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Libraries) != 0 {
		t.Errorf("expected 0 libraries due to error, got %d", len(resp.Libraries))
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0] != "FailingPlex: connection refused" {
		t.Errorf("error = %q, want 'FailingPlex: connection refused'", resp.Errors[0])
	}
}

func TestGetLibrariesDisabledServerSkipped(t *testing.T) {
	srv, st := newTestServer(t)
	srv.InvalidateLibraryCache()

	s := &models.Server{Name: "DisabledPlex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k1", Enabled: false}
	st.CreateServer(s)

	p := setupTestPoller(t, srv, st)

	p.AddServer(s.ID, &mockLibraryServer{
		name:      "DisabledPlex",
		srvType:   models.ServerTypePlex,
		libraries: []models.Library{{ID: "1", Name: "Should Not Appear"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LibrariesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Libraries) != 0 {
		t.Errorf("expected 0 libraries (server disabled), got %d", len(resp.Libraries))
	}
}

func TestGetLibrariesCaching(t *testing.T) {
	srv, st := newTestServer(t)
	srv.InvalidateLibraryCache()

	s := &models.Server{Name: "CachePlex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k1", Enabled: true}
	st.CreateServer(s)

	p := setupTestPoller(t, srv, st)

	mock := &mockLibraryServer{
		name:      "CachePlex",
		srvType:   models.ServerTypePlex,
		libraries: []models.Library{{ID: "1", Name: "Cached Library"}},
	}
	p.AddServer(s.ID, mock)

	// First request - should fetch
	req := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w.Code)
	}

	var resp1 LibrariesResponse
	json.NewDecoder(w.Body).Decode(&resp1)
	if len(resp1.Libraries) != 1 {
		t.Fatalf("first request: expected 1 library, got %d", len(resp1.Libraries))
	}

	// Modify mock to return different data
	mock.libraries = []models.Library{{ID: "2", Name: "New Library"}}

	// Second request - should return cached data
	req2 := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp2 LibrariesResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	// Should still get the old cached data
	if len(resp2.Libraries) != 1 {
		t.Fatalf("second request: expected 1 library, got %d", len(resp2.Libraries))
	}
	if resp2.Libraries[0].Name != "Cached Library" {
		t.Errorf("expected cached 'Cached Library', got %q", resp2.Libraries[0].Name)
	}

	// Invalidate and fetch again
	srv.InvalidateLibraryCache()

	req3 := httptest.NewRequest(http.MethodGet, "/api/libraries", nil)
	w3 := httptest.NewRecorder()
	srv.ServeHTTP(w3, req3)

	var resp3 LibrariesResponse
	json.NewDecoder(w3.Body).Decode(&resp3)

	if resp3.Libraries[0].Name != "New Library" {
		t.Errorf("after invalidation expected 'New Library', got %q", resp3.Libraries[0].Name)
	}
}
