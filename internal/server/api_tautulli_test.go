package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestGetTautulliSettings_Empty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/tautulli", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp tautulliSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.URL != "" {
		t.Fatalf("expected empty URL, got %q", resp.URL)
	}
	if resp.APIKey != "" {
		t.Fatalf("expected empty api_key, got %q", resp.APIKey)
	}
}

func TestGetTautulliSettings_MasksAPIKey(t *testing.T) {
	srv, st := newTestServer(t)

	if err := st.SetSetting("tautulli.url", "http://localhost:8181"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("tautulli.api_key", "supersecretkey"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/tautulli", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp tautulliSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.APIKey != maskedSecret {
		t.Fatalf("expected masked api_key %q, got %q", maskedSecret, resp.APIKey)
	}
	if resp.URL != "http://localhost:8181" {
		t.Fatalf("expected URL, got %q", resp.URL)
	}
}

func TestUpdateTautulliSettings_Saves(t *testing.T) {
	srv, st := newTestServer(t)

	body := `{"url":"http://tautulli:8181","api_key":"myapikey123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/tautulli", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetTautulliConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://tautulli:8181" {
		t.Fatalf("URL: got %q", cfg.URL)
	}
	if cfg.APIKey != "myapikey123" {
		t.Fatalf("APIKey: got %q", cfg.APIKey)
	}
}

func TestUpdateTautulliSettings_MaskedKeyPreservesExisting(t *testing.T) {
	srv, st := newTestServer(t)

	if err := st.SetSetting("tautulli.api_key", "original_key"); err != nil {
		t.Fatal(err)
	}

	body := `{"url":"http://new-host:8181","api_key":"********"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/tautulli", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetTautulliConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "original_key" {
		t.Fatalf("expected preserved API key, got %q", cfg.APIKey)
	}
}

func TestUpdateTautulliSettings_InvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/tautulli", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteTautulliSettings(t *testing.T) {
	srv, st := newTestServer(t)

	st.SetSetting("tautulli.url", "http://localhost:8181")
	st.SetSetting("tautulli.api_key", "somekey")

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/tautulli", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetTautulliConfig()
	if cfg.URL != "" || cfg.APIKey != "" {
		t.Fatalf("expected cleared config, got %+v", cfg)
	}
}

func TestTestTautulliConnection_MissingURL(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/test", strings.NewReader(`{"url":"","api_key":"key"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestTautulliConnection_MissingAPIKey(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/test", strings.NewReader(`{"url":"http://localhost:8181","api_key":""}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_MissingServerID(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_ServerNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(`{"server_id":999}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_NoTautulliConfigured(t *testing.T) {
	srv, st := newTestServer(t)

	plex := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(plex)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(fmt.Sprintf(`{"server_id":%d}`, plex.ID)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
