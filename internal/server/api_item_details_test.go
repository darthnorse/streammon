package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/auth"
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

func TestServerListRedactsURLForViewer(t *testing.T) {
	srv, st := newTestServer(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://internal:32400", APIKey: "k", MachineID: "secret-machine-id", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	viewerToken := createViewerSession(t, st, "viewer")

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var servers []models.Server
	if err := json.NewDecoder(w.Body).Decode(&servers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].URL != "" {
		t.Fatalf("expected URL to be redacted for viewer, got %q", servers[0].URL)
	}
	if servers[0].MachineID != "" {
		t.Fatalf("expected MachineID to be redacted for viewer, got %q", servers[0].MachineID)
	}
	if servers[0].Name != "Plex" {
		t.Fatalf("expected Name to be visible, got %q", servers[0].Name)
	}
}

func TestServerListShowsURLForAdmin(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://internal:32400", APIKey: "k", MachineID: "secret-machine-id", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var servers []models.Server
	if err := json.NewDecoder(w.Body).Decode(&servers); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].URL != "http://internal:32400" {
		t.Fatalf("expected URL to be visible for admin, got %q", servers[0].URL)
	}
	if servers[0].MachineID != "secret-machine-id" {
		t.Fatalf("expected MachineID to be visible for admin, got %q", servers[0].MachineID)
	}
}

func TestServerGetRedactsURLForViewer(t *testing.T) {
	srv, st := newTestServer(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://internal:32400", APIKey: "k", MachineID: "secret-machine-id", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	viewerToken := createViewerSession(t, st, "viewer")

	req := httptest.NewRequest(http.MethodGet, "/api/servers/1", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var server models.Server
	if err := json.NewDecoder(w.Body).Decode(&server); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if server.URL != "" {
		t.Fatalf("expected URL to be redacted for viewer, got %q", server.URL)
	}
	if server.MachineID != "" {
		t.Fatalf("expected MachineID to be redacted for viewer, got %q", server.MachineID)
	}
}
