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
		if r.Header.Get("X-Api-Key") != "test-api-key" {
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
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v3/series/"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": 10, "title": "Test Show", "year": 2024, "network": "HBO",
				"overview": "A test show overview", "status": "continuing",
				"genres": []string{"Drama"},
				"ratings": map[string]any{"value": 8.5},
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
		URL:     mockURL,
		APIKey:  "test-api-key",
		Enabled: true,
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

func TestSonarrIntegrationSettings(t *testing.T) {
	testIntegrationSettingsCRUD(t, integrationTestConfig{
		name:           "sonarr",
		settingsPath:   "/api/settings/sonarr",
		testPath:       "/api/settings/sonarr/test",
		configuredPath: "/api/sonarr/configured",
		dataPath:       "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07",
		configure:      configureSonarr,
		getConfig:      func(st *store.Store) (store.IntegrationConfig, error) { return st.GetSonarrConfig() },
		setConfig:      func(st *store.Store, c store.IntegrationConfig) error { return st.SetSonarrConfig(c) },
		mockServer:     mockSonarr,
	})
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

func failingSonarrTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	srv, st := newTestServerWrapped(t)
	st.SetSonarrConfig(store.SonarrConfig{URL: ts.URL, APIKey: "k", Enabled: true})
	return srv, st
}

func TestSonarrPoster_UpstreamError(t *testing.T) {
	srv, _ := failingSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (normalized), got %d: %s", w.Code, w.Body.String())
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

func TestSonarrSeries_Success(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/series/10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var series map[string]any
	json.NewDecoder(w.Body).Decode(&series)
	if series["title"] != "Test Show" {
		t.Fatalf("expected Test Show, got %v", series["title"])
	}
}

func TestSonarrSeries_InvalidID(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/series/abc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrSeries_NegativeID(t *testing.T) {
	srv, _ := newSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/series/-1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrSeries_UpstreamError(t *testing.T) {
	srv, _ := failingSonarrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/series/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrSeries_NotConfigured(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sonarr/series/10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSonarrEnabled_PosterDisabled(t *testing.T) {
	mock := mockSonarr(t)
	srv, st := newTestServerWrapped(t)
	configureSonarr(t, st, mock.URL)

	// Disable integration
	body := `{"url":"` + mock.URL + `","api_key":"` + maskedSecret + `","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/sonarr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Poster should return 503
	req = httptest.NewRequest(http.MethodGet, "/api/sonarr/poster/10", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for poster when disabled, got %d: %s", w.Code, w.Body.String())
	}
}
