package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
	"streammon/internal/store"
)

func mockTautulli(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "test-api-key" {
			json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"result":  "error",
					"message": "Invalid apikey",
				},
			})
			return
		}
		switch r.URL.Query().Get("cmd") {
		case "get_server_info":
			json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"result":  "success",
					"message": "",
					"data":    map[string]any{"pms_name": "Test"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func configureTautulli(t *testing.T, st *store.Store, mockURL string) {
	t.Helper()
	if err := st.SetTautulliConfig(store.TautulliConfig{
		URL:     mockURL,
		APIKey:  "test-api-key",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTautulliIntegrationSettings(t *testing.T) {
	testIntegrationSettingsCRUD(t, integrationTestConfig{
		name:           "tautulli",
		settingsPath:   "/api/settings/tautulli",
		testPath:       "/api/settings/tautulli/test",
		configure:      configureTautulli,
		getConfig:      func(st *store.Store) (store.IntegrationConfig, error) { return st.GetTautulliConfig() },
		setConfig:      func(st *store.Store, c store.IntegrationConfig) error { return st.SetTautulliConfig(c) },
		mockServer:     mockTautulli,
	})
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

	configureTautulli(t, st, "http://localhost:8181")

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
