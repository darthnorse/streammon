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
	srv, _ := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

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
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/tautulli", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteTautulliSettings(t *testing.T) {
	srv, st := newTestServerWrapped(t)

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
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/test", strings.NewReader(`{"url":"","api_key":"key"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestTautulliConnection_MissingAPIKey(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/test", strings.NewReader(`{"url":"http://localhost:8181","api_key":""}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_MissingServerID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_ServerNotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(`{"server_id":999}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTautulliImport_NoTautulliConfigured(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	plex := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(plex)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/import", strings.NewReader(fmt.Sprintf(`{"server_id":%d}`, plex.ID)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnrichStatus_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/tautulli/enrich/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp enrichmentStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Running {
		t.Fatal("expected not running")
	}
	if resp.Total != 0 {
		t.Fatalf("expected 0 total, got %d", resp.Total)
	}
}

func TestStartEnrich_MissingServerID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/enrich", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartEnrich_NoTautulliConfigured(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	plex := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(plex)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/enrich", strings.NewReader(fmt.Sprintf(`{"server_id":%d}`, plex.ID)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartEnrich_NoneToEnrich(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetSetting("tautulli.url", "http://localhost:8181")
	st.SetSetting("tautulli.api_key", "testkey")

	plex := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(plex)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/enrich",
		strings.NewReader(fmt.Sprintf(`{"server_id":%d}`, plex.ID)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp tautulliEnrichResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "none" {
		t.Fatalf("expected status 'none', got %q", resp.Status)
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
}

func TestStopEnrich_WhenNotRunning(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/tautulli/enrich/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEnrichStatus_WithServerIDParam(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/tautulli/enrich/status?server_id=1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp enrichmentStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ServerID != 1 {
		t.Fatalf("expected server_id 1, got %d", resp.ServerID)
	}
}
