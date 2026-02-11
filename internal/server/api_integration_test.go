package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/store"
)

type integrationTestConfig struct {
	name           string
	settingsPath   string
	testPath       string
	configuredPath string
	dataPath       string // an endpoint that returns 503 when not configured
	configure      func(*testing.T, *store.Store, string)
	getConfig      func(*store.Store) (store.IntegrationConfig, error)
	setConfig      func(*store.Store, store.IntegrationConfig) error
	mockServer     func(*testing.T) *httptest.Server
}

func testIntegrationSettingsCRUD(t *testing.T, cfg integrationTestConfig) {
	t.Helper()

	t.Run("GetEmpty", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, cfg.settingsPath, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp integrationSettings
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.URL != "" || resp.APIKey != "" {
			t.Fatalf("expected empty settings, got %+v", resp)
		}
	})

	t.Run("UpdateSaves", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)

		body := `{"url":"` + mock.URL + `","api_key":"test-key-123"}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		icfg, err := cfg.getConfig(st)
		if err != nil {
			t.Fatal(err)
		}
		if icfg.URL != mock.URL {
			t.Fatalf("URL: got %q", icfg.URL)
		}
		if icfg.APIKey != "test-key-123" {
			t.Fatalf("APIKey: got %q", icfg.APIKey)
		}
	})

	t.Run("MasksKey", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		req := httptest.NewRequest(http.MethodGet, cfg.settingsPath, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp integrationSettings
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.APIKey != maskedSecret {
			t.Fatalf("expected masked api_key %q, got %q", maskedSecret, resp.APIKey)
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"url":"not-a-url","api_key":"key"}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("RequiresKeyOnURLChange", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		body := `{"url":"http://new-host:9999","api_key":"` + maskedSecret + `"}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when URL changes without new key, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("MaskedKeyPreserved", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `"}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		icfg, _ := cfg.getConfig(st)
		if icfg.APIKey == "" || icfg.APIKey == maskedSecret {
			t.Fatalf("expected preserved key, got %q", icfg.APIKey)
		}
	})

	t.Run("DeleteSettings", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		req := httptest.NewRequest(http.MethodDelete, cfg.settingsPath, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}

		icfg, _ := cfg.getConfig(st)
		if icfg.URL != "" || icfg.APIKey != "" {
			t.Fatalf("expected empty URL/APIKey after delete, got %+v", icfg)
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader("not json"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(""))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("TestConnection_Success", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, _ := newTestServerWrapped(t)

		body := `{"url":"` + mock.URL + `","api_key":"test-api-key"}`
		req := httptest.NewRequest(http.MethodPost, cfg.testPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp integrationTestResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success {
			t.Fatalf("expected success, got error: %s", resp.Error)
		}
	})

	t.Run("TestConnection_Failure", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, _ := newTestServerWrapped(t)

		body := `{"url":"` + mock.URL + `","api_key":"wrong-key"}`
		req := httptest.NewRequest(http.MethodPost, cfg.testPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp integrationTestResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Success {
			t.Fatal("expected failure for wrong key")
		}
	})

	t.Run("TestConnection_MissingURL", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"api_key":"key"}`
		req := httptest.NewRequest(http.MethodPost, cfg.testPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("TestConnection_MalformedJSON", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPost, cfg.testPath, strings.NewReader("{bad"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("TestConnection_FallsBackToStoredKey", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `"}`
		req := httptest.NewRequest(http.MethodPost, cfg.testPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp integrationTestResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success {
			t.Fatalf("expected success with stored key, got error: %s", resp.Error)
		}
	})

	if cfg.configuredPath != "" {
		t.Run("Configured_True", func(t *testing.T) {
			mock := cfg.mockServer(t)
			srv, st := newTestServerWrapped(t)
			cfg.configure(t, st, mock.URL)

			req := httptest.NewRequest(http.MethodGet, cfg.configuredPath, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var resp map[string]bool
			json.NewDecoder(w.Body).Decode(&resp)
			if !resp["configured"] {
				t.Fatal("expected configured=true")
			}
		})

		t.Run("Configured_False", func(t *testing.T) {
			srv, _ := newTestServerWrapped(t)

			req := httptest.NewRequest(http.MethodGet, cfg.configuredPath, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var resp map[string]bool
			json.NewDecoder(w.Body).Decode(&resp)
			if resp["configured"] {
				t.Fatal("expected configured=false")
			}
		})
	}

	t.Run("Enabled_DisableViaUpdate", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `","enabled":false}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Configured should return false
		if cfg.configuredPath != "" {
			req = httptest.NewRequest(http.MethodGet, cfg.configuredPath, nil)
			w = httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			var resp map[string]bool
			json.NewDecoder(w.Body).Decode(&resp)
			if resp["configured"] {
				t.Fatal("expected configured=false after disabling")
			}
		}

		// Data endpoint should return 503
		if cfg.dataPath != "" {
			req = httptest.NewRequest(http.MethodGet, cfg.dataPath, nil)
			w = httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503 when disabled, got %d: %s", w.Code, w.Body.String())
			}
		}
	})

	t.Run("Enabled_ReEnable", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		// Disable
		body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `","enabled":false}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("disable: expected 200, got %d", w.Code)
		}

		// Re-enable
		body = `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `","enabled":true}`
		req = httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w = httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("re-enable: expected 200, got %d", w.Code)
		}

		// Configured should be true again
		if cfg.configuredPath != "" {
			req = httptest.NewRequest(http.MethodGet, cfg.configuredPath, nil)
			w = httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			var resp map[string]bool
			json.NewDecoder(w.Body).Decode(&resp)
			if !resp["configured"] {
				t.Fatal("expected configured=true after re-enabling")
			}
		}
	})

	t.Run("Enabled_OmittedPreservesState", func(t *testing.T) {
		mock := cfg.mockServer(t)
		srv, st := newTestServerWrapped(t)
		cfg.configure(t, st, mock.URL)

		// Disable explicitly
		body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `","enabled":false}`
		req := httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("disable: expected 200, got %d", w.Code)
		}

		// Update without enabled field — should preserve disabled state
		body = `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `"}`
		req = httptest.NewRequest(http.MethodPut, cfg.settingsPath, strings.NewReader(body))
		w = httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("update: expected 200, got %d", w.Code)
		}

		icfg, _ := cfg.getConfig(st)
		if icfg.Enabled {
			t.Fatal("expected Enabled to remain false when omitted from update")
		}
	})

	t.Run("Enabled_GetReturnsState", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		cfg.setConfig(st, store.IntegrationConfig{URL: "http://host:8080", APIKey: "k", Enabled: false})

		req := httptest.NewRequest(http.MethodGet, cfg.settingsPath, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp integrationSettings
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Enabled == nil || *resp.Enabled {
			t.Fatal("expected enabled=false in GET response")
		}
	})

	t.Run("ViewerForbidden", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-"+cfg.name)

		tests := []struct {
			name   string
			method string
			path   string
			body   string
		}{
			{"get settings", http.MethodGet, cfg.settingsPath, ""},
			{"update settings", http.MethodPut, cfg.settingsPath, `{"url":"http://x","api_key":"k"}`},
			{"delete settings", http.MethodDelete, cfg.settingsPath, ""},
			{"test connection", http.MethodPost, cfg.testPath, `{"url":"http://x","api_key":"k"}`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var req *http.Request
				if tt.body != "" {
					req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				} else {
					req = httptest.NewRequest(tt.method, tt.path, nil)
				}
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
				w := httptest.NewRecorder()
				srv.ServeHTTP(w, req)

				if w.Code != http.StatusForbidden {
					t.Fatalf("expected 403 for viewer on %s, got %d: %s", tt.name, w.Code, w.Body.String())
				}
			})
		}
	})
}
