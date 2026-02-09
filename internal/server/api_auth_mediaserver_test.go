package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/models"
	"streammon/internal/store"
)

// fakeEmbyServer returns a test HTTP server that mimics Emby/Jellyfin AuthenticateByName.
func fakeEmbyServer(t *testing.T, wantUser, wantPass string, isAdmin bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/AuthenticateByName" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Username string `json:"Username"`
			Pw       string `json:"Pw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.Username != wantUser || body.Pw != wantPass {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"User": map[string]interface{}{
				"Id":   "abc-123",
				"Name": wantUser,
				"Policy": map[string]interface{}{
					"IsAdministrator": isAdmin,
				},
			},
			"AccessToken": "fake-token",
		})
	}))
}

// newEmbyTestServer creates a test server with local + emby providers and a pre-existing admin.
func newEmbyTestServer(t *testing.T, fakeURL string) (*testServer, *store.Store) {
	t.Helper()
	st := newEmptyStore(t)
	st.CreateLocalUser("admin", "", "", models.RoleAdmin)
	authMgr := auth.NewManager(st)
	authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
	authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
	srv := NewServer(st, WithAuthManager(authMgr))
	return &testServer{srv}, st
}

func TestMediaServerLogin(t *testing.T) {
	t.Run("successful login", func(t *testing.T) {
		fake := fakeEmbyServer(t, "alice", "secret", false)
		defer fake.Close()

		wrapped, st := newEmbyTestServer(t, fake.URL)

		if err := st.SetGuestAccess(true); err != nil {
			t.Fatal(err)
		}

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		if err := st.CreateServer(embyServer); err != nil {
			t.Fatal(err)
		}

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"alice","password":"secret"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/emby/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var user models.User
		if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
			t.Fatalf("decode user: %v", err)
		}
		if user.Name != "alice" {
			t.Fatalf("expected name alice, got %s", user.Name)
		}
	})

	t.Run("invalid credentials returns 401", func(t *testing.T) {
		fake := fakeEmbyServer(t, "alice", "secret", false)
		defer fake.Close()

		wrapped, st := newEmbyTestServer(t, fake.URL)

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(embyServer)

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"alice","password":"wrong"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/emby/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("guest access disabled blocks non-admin", func(t *testing.T) {
		fake := fakeEmbyServer(t, "viewer", "pass", false)
		defer fake.Close()

		wrapped, st := newEmbyTestServer(t, fake.URL)

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(embyServer)

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"viewer","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/emby/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("server admin bypasses guest access check", func(t *testing.T) {
		fake := fakeEmbyServer(t, "admin", "pass", true)
		defer fake.Close()

		wrapped, st := newEmbyTestServer(t, fake.URL)

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(embyServer)

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"admin","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/emby/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong server type returns 400", func(t *testing.T) {
		fake := fakeEmbyServer(t, "alice", "secret", false)
		defer fake.Close()

		wrapped, st := newEmbyTestServer(t, fake.URL)

		// Need at least one emby server so the provider is Enabled
		st.CreateServer(&models.Server{Name: "RealEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true})
		jfServer := &models.Server{Name: "TestJF", Type: models.ServerTypeJellyfin, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(jfServer)

		body := `{"server_id":` + sid(jfServer.ID) + `,"username":"alice","password":"secret"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/emby/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestMediaServerGetServers(t *testing.T) {
	t.Run("returns only enabled servers of correct type", func(t *testing.T) {
		st := newEmptyStore(t)
		st.CreateLocalUser("admin", "", "", models.RoleAdmin)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeJellyfin))
		newsrv := NewServer(st, WithAuthManager(authMgr))
		wrapped := &testServer{newsrv}

		st.CreateServer(&models.Server{Name: "Emby1", Type: models.ServerTypeEmby, URL: "http://emby1", APIKey: "k", Enabled: true})
		st.CreateServer(&models.Server{Name: "Emby2", Type: models.ServerTypeEmby, URL: "http://emby2", APIKey: "k", Enabled: false})
		st.CreateServer(&models.Server{Name: "JF1", Type: models.ServerTypeJellyfin, URL: "http://jf1", APIKey: "k", Enabled: true})

		req := httptest.NewRequest(http.MethodGet, "/auth/emby/servers", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var servers []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(w.Body).Decode(&servers); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(servers) != 1 {
			t.Fatalf("expected 1 emby server, got %d", len(servers))
		}
		if servers[0].Name != "Emby1" {
			t.Fatalf("expected Emby1, got %s", servers[0].Name)
		}
	})

	t.Run("jellyfin servers endpoint returns jellyfin only", func(t *testing.T) {
		st := newEmptyStore(t)
		st.CreateLocalUser("admin", "", "", models.RoleAdmin)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeJellyfin))
		newsrv := NewServer(st, WithAuthManager(authMgr))
		wrapped := &testServer{newsrv}

		st.CreateServer(&models.Server{Name: "Emby1", Type: models.ServerTypeEmby, URL: "http://emby1", APIKey: "k", Enabled: true})
		st.CreateServer(&models.Server{Name: "JF1", Type: models.ServerTypeJellyfin, URL: "http://jf1", APIKey: "k", Enabled: true})

		req := httptest.NewRequest(http.MethodGet, "/auth/jellyfin/servers", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var servers []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		json.NewDecoder(w.Body).Decode(&servers)
		if len(servers) != 1 {
			t.Fatalf("expected 1 jellyfin server, got %d", len(servers))
		}
		if servers[0].Name != "JF1" {
			t.Fatalf("expected JF1, got %s", servers[0].Name)
		}
	})
}

func TestMediaServerProviderEnabled(t *testing.T) {
	t.Run("disabled when no servers of type", func(t *testing.T) {
		st := newEmptyStore(t)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
		newsrv := NewServer(st, WithAuthManager(authMgr))
		wrapped := &testServer{newsrv}

		req := httptest.NewRequest(http.MethodGet, "/auth/providers", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		var providers []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		json.NewDecoder(w.Body).Decode(&providers)

		for _, p := range providers {
			if p.Name == "emby" && p.Enabled {
				t.Fatal("expected emby to be disabled when no servers configured")
			}
		}
	})

	t.Run("enabled when server of type exists", func(t *testing.T) {
		st := newEmptyStore(t)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
		newsrv := NewServer(st, WithAuthManager(authMgr))
		wrapped := &testServer{newsrv}

		st.CreateServer(&models.Server{Name: "Emby1", Type: models.ServerTypeEmby, URL: "http://emby1", APIKey: "k", Enabled: true})

		req := httptest.NewRequest(http.MethodGet, "/auth/providers", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		var providers []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		json.NewDecoder(w.Body).Decode(&providers)

		found := false
		for _, p := range providers {
			if p.Name == "emby" && p.Enabled {
				found = true
			}
		}
		if !found {
			t.Fatal("expected emby to be enabled when server configured")
		}
	})
}

func TestMediaServerSetup(t *testing.T) {
	t.Run("first admin via emby", func(t *testing.T) {
		fake := fakeEmbyServer(t, "admin", "pass", true)
		defer fake.Close()

		st := newEmptyStore(t)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
		newsrv := NewServer(st, WithAuthManager(authMgr))

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(embyServer)

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"admin","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/emby", strings.NewReader(body))
		w := httptest.NewRecorder()
		newsrv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var user models.User
		json.NewDecoder(w.Body).Decode(&user)
		if user.Role != models.RoleAdmin {
			t.Fatalf("expected admin role, got %s", user.Role)
		}
	})

	t.Run("non-admin cannot setup", func(t *testing.T) {
		fake := fakeEmbyServer(t, "viewer", "pass", false)
		defer fake.Close()

		st := newEmptyStore(t)
		authMgr := auth.NewManager(st)
		authMgr.RegisterProvider(auth.NewLocalProvider(st, authMgr))
		authMgr.RegisterProvider(auth.NewMediaServerProvider(st, authMgr, models.ServerTypeEmby))
		newsrv := NewServer(st, WithAuthManager(authMgr))

		embyServer := &models.Server{Name: "TestEmby", Type: models.ServerTypeEmby, URL: fake.URL, APIKey: "key", Enabled: true}
		st.CreateServer(embyServer)

		body := `{"server_id":` + sid(embyServer.ID) + `,"username":"viewer","password":"pass"}`
		req := httptest.NewRequest(http.MethodPost, "/api/setup/emby", strings.NewReader(body))
		w := httptest.NewRecorder()
		newsrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func sid(n int64) string {
	return fmt.Sprintf("%d", n)
}
