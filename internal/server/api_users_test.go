package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

type stubResolver struct {
	results map[string]*models.GeoResult
}

func (s *stubResolver) Lookup(ip net.IP) *models.GeoResult {
	if ip == nil {
		return nil
	}
	return s.results[ip.String()]
}

func TestListUsersAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.GetOrCreateUser("alice")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var users []models.User
	json.NewDecoder(w.Body).Decode(&users)
	// 2 users: test-admin (from test setup) + alice
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestGetUserAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.GetOrCreateUser("alice")

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)
	if user.Name != "alice" {
		t.Fatalf("expected alice, got %s", user.Name)
	}
}

func TestGetUserNotFoundAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/nobody", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetUserLocationsAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"})

	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: 1, UserName: "alice", MediaType: "movie", Title: "A",
		IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var locs []models.GeoResult
	json.NewDecoder(w.Body).Decode(&locs)
	if locs == nil {
		t.Fatal("expected [], got null")
	}
}

func TestGetUserLocationsNoHistoryAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/nobody/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserLocationsCachedAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"})

	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: 1, UserName: "alice", MediaType: "movie", Title: "A",
		IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now,
	})

	st.SetCachedGeo(&models.GeoResult{
		IP: "8.8.8.8", Lat: 37.386, Lng: -122.084, City: "Mountain View", Country: "US",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var locs []models.GeoResult
	json.NewDecoder(w.Body).Decode(&locs)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	if locs[0].City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", locs[0].City)
	}
}

func TestGetUserLocationsResolverLookupAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	srv.Unwrap().geoResolver = &stubResolver{results: map[string]*models.GeoResult{
		"8.8.8.8": {IP: "8.8.8.8", Lat: 37.386, Lng: -122.084, City: "Mountain View", Country: "US"},
	}}

	st.CreateServer(&models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"})

	now := time.Now().UTC()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: 1, UserName: "alice", MediaType: "movie", Title: "A",
		IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now,
	})

	// First request: resolver lookup, should cache the result
	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var locs []models.GeoResult
	json.NewDecoder(w.Body).Decode(&locs)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	if locs[0].City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", locs[0].City)
	}

	// Verify cache was populated
	cached, err := st.GetCachedGeo("8.8.8.8")
	if err != nil {
		t.Fatalf("GetCachedGeo: %v", err)
	}
	if cached == nil {
		t.Fatal("expected resolver result to be cached")
	}
	if cached.City != "Mountain View" {
		t.Fatalf("expected cached Mountain View, got %s", cached.City)
	}
}

func TestSyncUserAvatars_NoServers(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/users/sync-avatars", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SyncUserAvatarsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Synced != 0 || resp.Updated != 0 {
		t.Errorf("expected synced=0 updated=0, got synced=%d updated=%d", resp.Synced, resp.Updated)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected no errors, got %v", resp.Errors)
	}
}

func TestSyncUserAvatars_DisabledServersSkipped(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "DisabledPlex", Type: models.ServerTypePlex,
		URL: "http://localhost:9999", APIKey: "k", Enabled: false,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users/sync-avatars", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SyncUserAvatarsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Disabled server should be skipped, no errors
	if len(resp.Errors) != 0 {
		t.Errorf("expected no errors (disabled server skipped), got %v", resp.Errors)
	}
}

func TestSyncUserAvatars_ServerConnectionError(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "BadPlex", Type: models.ServerTypePlex,
		URL: "http://localhost:9999", APIKey: "k", Enabled: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users/sync-avatars", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should still return 200 with errors in body
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SyncUserAvatarsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Errors) != 1 {
		t.Errorf("expected 1 error for unreachable server, got %d: %v", len(resp.Errors), resp.Errors)
	}
}

func TestListUserSummaries_ViewerForbidden(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	viewerToken := createViewerSession(t, st, "viewer")

	req := httptest.NewRequest(http.MethodGet, "/api/users/summary", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.Server.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("viewer hitting /api/users/summary: status = %d, want 403", w.Code)
	}
}

func TestListUserSummaries_AdminAllowed(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/summary", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("admin hitting /api/users/summary: status = %d, want 200", w.Code)
	}
}

func TestUserNotesAPI_RoundTrip(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	if _, err := st.GetOrCreateUser("alice"); err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/users/alice/notes",
		strings.NewReader(`{"notes":"brother of patrik"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users/alice/notes", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", w.Code)
	}
	var resp struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Notes != "brother of patrik" {
		t.Fatalf("expected note, got %q", resp.Notes)
	}
}

func TestUserNotesAPI_UpsertsAbsentUser(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPut, "/api/users/ghost/notes",
		strings.NewReader(`{"notes":"cryptic media user"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users/ghost/notes", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Notes != "cryptic media user" {
		t.Fatalf("expected upserted note, got %q", resp.Notes)
	}
}

func TestUserNotesAPI_ForbiddenForViewer(t *testing.T) {
	srv, st := newTestServer(t) // unwrapped: no auto admin cookie
	viewerToken := createViewerSession(t, st, "vic")

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/notes", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUserNotesAPI_TooLong(t *testing.T) {
	srv, _ := newTestServerWrapped(t)
	body := `{"notes":"` + strings.Repeat("a", 5001) + `"}`

	req := httptest.NewRequest(http.MethodPut, "/api/users/alice/notes",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
