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

func mockSonarr(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-sonarr-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/system/status":
			json.NewEncoder(w).Encode(map[string]string{"version": "4.0.0"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/calendar":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id": 1, "seriesId": 10, "seasonNumber": 1, "episodeNumber": 1,
					"title": "Pilot", "airDateUtc": "2025-03-01T20:00:00Z", "airDate": "2025-03-01",
					"hasFile": false, "monitored": true,
					"series": map[string]any{"id": 10, "title": "Test Show"},
				},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v3/mediacover/"):
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("fake-image-data"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func configureSonarr(t *testing.T, st *store.Store, mockURL string) {
	t.Helper()
	if err := st.SetSonarrConfig(store.SonarrConfig{
		URL:    mockURL,
		APIKey: "test-sonarr-key",
	}); err != nil {
		t.Fatal(err)
	}
}

func newSonarrTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	mock := mockSonarr(t)
	srv, st := newTestServerWrapped(t)
	configureSonarr(t, st, mock.URL)
	return srv, st
}

func TestGetSonarrSettings_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/sonarr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sonarrSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.URL != "" || resp.APIKey != "" {
		t.Fatalf("expected empty settings, got %+v", resp)
	}
}

func TestUpdateSonarrSettings_Saves(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	body := `{"url":"http://sonarr:8989","api_key":"mykey123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetSonarrConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://sonarr:8989" {
		t.Fatalf("URL: got %q", cfg.URL)
	}
	if cfg.APIKey != "mykey123" {
		t.Fatalf("APIKey: got %q", cfg.APIKey)
	}
}

func TestGetSonarrSettings_MasksKey(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetSonarrConfig(store.SonarrConfig{URL: "http://sonarr:8989", APIKey: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/sonarr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp sonarrSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.APIKey != maskedSecret {
		t.Fatalf("expected masked api_key %q, got %q", maskedSecret, resp.APIKey)
	}
}

func TestUpdateSonarrSettings_InvalidURL(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"url":"not-a-url","api_key":"key"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSonarrSettings_RequiresKeyOnURLChange(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetSonarrConfig(store.SonarrConfig{URL: "http://old:8989", APIKey: "oldkey"})

	// Change URL but send masked key (no new key)
	body := `{"url":"http://new:8989","api_key":"` + maskedSecret + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when URL changes without new key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSonarrSettings_MaskedKeyPreserved(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetSonarrConfig(store.SonarrConfig{URL: "http://sonarr:8989", APIKey: "original"})

	// Same URL, masked key — should preserve existing key
	body := `{"url":"http://sonarr:8989","api_key":"` + maskedSecret + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetSonarrConfig()
	if cfg.APIKey != "original" {
		t.Fatalf("expected preserved key %q, got %q", "original", cfg.APIKey)
	}
}

func TestDeleteSonarrSettings(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetSonarrConfig(store.SonarrConfig{URL: "http://sonarr:8989", APIKey: "key"})

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/sonarr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cfg, _ := st.GetSonarrConfig()
	if cfg.URL != "" || cfg.APIKey != "" {
		t.Fatalf("expected empty config after delete, got %+v", cfg)
	}
}

func TestSonarrTestConnection_Success(t *testing.T) {
	mock := mockSonarr(t)
	srv, _ := newTestServerWrapped(t)

	body := `{"url":"` + mock.URL + `","api_key":"test-sonarr-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sonarr/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sonarrTestResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
}

func TestSonarrTestConnection_Failure(t *testing.T) {
	mock := mockSonarr(t)
	srv, _ := newTestServerWrapped(t)

	body := `{"url":"` + mock.URL + `","api_key":"wrong-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sonarr/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sonarrTestResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Fatal("expected failure for wrong key")
	}
}

func TestSonarrTestConnection_MissingURL(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"api_key":"key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sonarr/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrTestConnection_FallsBackToStoredKey(t *testing.T) {
	mock := mockSonarr(t)
	srv, st := newTestServerWrapped(t)
	configureSonarr(t, st, mock.URL)

	// Send masked key — should use stored key
	body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sonarr/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp sonarrTestResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected success with stored key, got error: %s", resp.Error)
	}
}

func TestSonarrConfigured_True(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.SetSonarrConfig(store.SonarrConfig{URL: "http://sonarr:8989", APIKey: "key"})

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/configured", nil)
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
}

func TestSonarrConfigured_False(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/configured", nil)
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
}

func TestSonarrConfigured_ViewerCanAccess(t *testing.T) {
	srv, st := newTestServer(t)
	configureSonarr(t, st, "http://sonarr:8989")

	viewerToken := createViewerSession(t, st, "viewer-sonarr-cfg")

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/configured", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrCalendar_Success(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var episodes []map[string]any
	json.NewDecoder(w.Body).Decode(&episodes)
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
}

func TestSonarrCalendar_NotConfigured(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrCalendar_InvalidDateFormat(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	tests := []struct {
		name  string
		query string
	}{
		{"invalid start", "start=not-a-date&end=2025-03-07"},
		{"invalid end", "start=2025-03-01&end=bad"},
		{"both invalid", "start=01/03/2025&end=07/03/2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?"+tt.query, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestSonarrCalendar_EmptyDatesAllowed(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with no dates, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrPoster_Success(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Fatalf("expected Content-Type image/jpeg, got %q", ct)
	}

	cache := w.Header().Get("Cache-Control")
	if cache != "public, max-age=14400" {
		t.Fatalf("expected 4h cache, got %q", cache)
	}
}

func TestSonarrPoster_InvalidSeriesID(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/abc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrPoster_NotConfigured(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrPoster_UpstreamError(t *testing.T) {
	// Use a mock that returns 404 for poster requests
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	srv, st := newTestServerWrapped(t)
	st.SetSonarrConfig(store.SonarrConfig{URL: ts.URL, APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should be normalized to 502, not the upstream 404
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (normalized), got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSonarrSettings_MalformedJSON(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSonarrSettings_EmptyBody(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(""))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrTestConnection_MalformedJSON(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/sonarr/test", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrCalendar_ViewerCanAccess(t *testing.T) {
	mock := mockSonarr(t)
	srv, st := newTestServer(t)
	configureSonarr(t, st, mock.URL)

	viewerToken := createViewerSession(t, st, "viewer-sonarr-cal")

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for viewer, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrSettings_ViewerForbidden(t *testing.T) {
	srv, st := newTestServer(t)
	viewerToken := createViewerSession(t, st, "viewer-sonarr")

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get settings", http.MethodGet, "/api/settings/sonarr", ""},
		{"update settings", http.MethodPut, "/api/settings/sonarr", `{"url":"http://x","api_key":"k"}`},
		{"delete settings", http.MethodDelete, "/api/settings/sonarr", ""},
		{"test connection", http.MethodPost, "/api/settings/sonarr/test", `{"url":"http://x","api_key":"k"}`},
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
}
