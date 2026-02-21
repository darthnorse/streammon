package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

// overseerrMedia configures what the mock Overseerr server returns for a media lookup.
type overseerrMedia struct {
	lookupPath string // e.g. "/api/v1/movie/27205"
	mediaID    int    // media entry ID (0 to omit)
	requestID  int    // request ID (0 to omit)
}

// newOverseerrServer creates a mock Overseerr server that handles media lookup,
// request deletion, and media clearing based on the given config.
func newOverseerrServer(t *testing.T, m overseerrMedia, requestDeleted, mediaCleared *atomic.Bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == m.lookupPath && r.Method == http.MethodGet:
			mi := map[string]any{}
			if m.mediaID != 0 {
				mi["id"] = m.mediaID
			}
			reqs := []map[string]any{}
			if m.requestID != 0 {
				reqs = append(reqs, map[string]any{"id": m.requestID})
			}
			mi["requests"] = reqs
			json.NewEncoder(w).Encode(map[string]any{"mediaInfo": mi})
		case m.requestID != 0 && r.URL.Path == fmt.Sprintf("/api/v1/request/%d", m.requestID) && r.Method == http.MethodDelete:
			if requestDeleted != nil {
				requestDeleted.Store(true)
			}
			w.WriteHeader(http.StatusNoContent)
		case m.mediaID != 0 && r.URL.Path == fmt.Sprintf("/api/v1/media/%d", m.mediaID) && r.Method == http.MethodDelete:
			if mediaCleared != nil {
				mediaCleared.Store(true)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func configureIntegration(t *testing.T, s *store.Store, prefix, url string) {
	t.Helper()
	cfg := store.IntegrationConfig{URL: url, APIKey: "test-key", Enabled: true}
	var err error
	switch prefix {
	case "radarr":
		err = s.SetRadarrConfig(cfg)
	case "sonarr":
		err = s.SetSonarrConfig(cfg)
	case "overseerr":
		err = s.SetOverseerrConfig(cfg)
	}
	if err != nil {
		t.Fatalf("set %s config: %v", prefix, err)
	}
}

func TestDeleteExternalReferences_MovieWithRadarrAndOverseerr(t *testing.T) {
	var radarrDeleted atomic.Bool
	radarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]any{{"id": 42}})
		case r.URL.Path == "/api/v3/movie/42" && r.Method == http.MethodDelete:
			radarrDeleted.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer radarrSrv.Close()

	var overseerrRequestDeleted, overseerrMediaCleared atomic.Bool
	overseerrSrv := newOverseerrServer(t, overseerrMedia{
		lookupPath: "/api/v1/movie/27205", mediaID: 42, requestID: 10,
	}, &overseerrRequestDeleted, &overseerrMediaCleared)
	defer overseerrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "radarr", radarrSrv.URL)
	configureIntegration(t, s, "overseerr", overseerrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Inception",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "27205",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	if !radarrDeleted.Load() {
		t.Error("expected Radarr movie to be deleted")
	}
	if !overseerrRequestDeleted.Load() {
		t.Error("expected Overseerr request to be deleted")
	}
	if !overseerrMediaCleared.Load() {
		t.Error("expected Overseerr media data to be cleared")
	}

	radarrResult := findResult(results, "radarr")
	if radarrResult == nil || !radarrResult.Success {
		t.Errorf("expected radarr success, got %+v", radarrResult)
	}
	overseerrResult := findResult(results, "overseerr")
	if overseerrResult == nil || !overseerrResult.Success {
		t.Errorf("expected overseerr success, got %+v", overseerrResult)
	}
}

func TestDeleteExternalReferences_TVWithSonarrAndOverseerr(t *testing.T) {
	var sonarrDeleted atomic.Bool
	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/series" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]any{{"id": 77}})
		case r.URL.Path == "/api/v3/series/77" && r.Method == http.MethodDelete:
			sonarrDeleted.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sonarrSrv.Close()

	var overseerrRequestDeleted, overseerrMediaCleared atomic.Bool
	overseerrSrv := newOverseerrServer(t, overseerrMedia{
		lookupPath: "/api/v1/tv/12345", mediaID: 55, requestID: 20,
	}, &overseerrRequestDeleted, &overseerrMediaCleared)
	defer overseerrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "sonarr", sonarrSrv.URL)
	configureIntegration(t, s, "overseerr", overseerrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Breaking Bad",
		MediaType: models.MediaTypeTV,
		TMDBID:    "12345",
		TVDBID:    "67890",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	if !sonarrDeleted.Load() {
		t.Error("expected Sonarr series to be deleted")
	}
	if !overseerrRequestDeleted.Load() {
		t.Error("expected Overseerr request to be deleted")
	}
	if !overseerrMediaCleared.Load() {
		t.Error("expected Overseerr media data to be cleared")
	}

	sonarrResult := findResult(results, "sonarr")
	if sonarrResult == nil || !sonarrResult.Success {
		t.Errorf("expected sonarr success, got %+v", sonarrResult)
	}
}

func TestDeleteExternalReferences_MediaOnlyNoRequest(t *testing.T) {
	var mediaCleared atomic.Bool
	overseerrSrv := newOverseerrServer(t, overseerrMedia{
		lookupPath: "/api/v1/movie/27205", mediaID: 42,
	}, nil, &mediaCleared)
	defer overseerrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "overseerr", overseerrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Inception",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "27205",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	if !mediaCleared.Load() {
		t.Error("expected Overseerr media data to be cleared")
	}
	overseerrResult := findResult(results, "overseerr")
	if overseerrResult == nil || !overseerrResult.Success {
		t.Errorf("expected overseerr success, got %+v", overseerrResult)
	}
}

func TestDeleteExternalReferences_RequestOnlyNoMediaID(t *testing.T) {
	var requestDeleted atomic.Bool
	overseerrSrv := newOverseerrServer(t, overseerrMedia{
		lookupPath: "/api/v1/movie/27205", requestID: 10,
	}, &requestDeleted, nil)
	defer overseerrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "overseerr", overseerrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Inception",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "27205",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	if !requestDeleted.Load() {
		t.Error("expected Overseerr request to be deleted")
	}
	overseerrResult := findResult(results, "overseerr")
	if overseerrResult == nil || !overseerrResult.Success {
		t.Errorf("expected overseerr success, got %+v", overseerrResult)
	}
}

func TestDeleteExternalReferences_NoExternalIDs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Unknown Movie",
		MediaType: models.MediaTypeMovie,
	}

	results := cd.DeleteExternalReferences(context.Background(), item)
	if len(results) != 0 {
		t.Errorf("expected no results for item without external IDs, got %d", len(results))
	}
}

func TestDeleteExternalReferences_IntegrationNotConfigured(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Inception",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "27205",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	// Should produce results but with no success (not configured = skip)
	for _, r := range results {
		if r.Success {
			t.Errorf("expected no success for unconfigured %s", r.Service)
		}
	}
}

func TestDeleteExternalReferences_NotFoundInExternalService(t *testing.T) {
	radarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/movie" {
			json.NewEncoder(w).Encode([]map[string]any{}) // empty result
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer radarrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "radarr", radarrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Unknown Movie",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "99999",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	radarrResult := findResult(results, "radarr")
	if radarrResult == nil {
		t.Fatal("expected radarr result")
	}
	if radarrResult.Success {
		t.Error("expected no success for movie not found in Radarr")
	}
	if radarrResult.Error != "" {
		t.Errorf("expected no error for not-found, got %s", radarrResult.Error)
	}
}

func TestDeleteExternalReferences_ExternalServiceError(t *testing.T) {
	radarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"server error"}`))
	}))
	defer radarrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "radarr", radarrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Inception",
		MediaType: models.MediaTypeMovie,
		TMDBID:    "27205",
	}

	results := cd.DeleteExternalReferences(context.Background(), item)

	radarrResult := findResult(results, "radarr")
	if radarrResult == nil {
		t.Fatal("expected radarr result")
	}
	if radarrResult.Success {
		t.Error("expected failure for server error")
	}
	if radarrResult.Error == "" {
		t.Error("expected error message for server error")
	}
}

func TestDeleteExternalReferences_DBRoundTrip(t *testing.T) {
	var radarrDeleted atomic.Bool
	radarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]any{{"id": 42}})
		case r.URL.Path == "/api/v3/movie/42" && r.Method == http.MethodDelete:
			radarrDeleted.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer radarrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "radarr", radarrSrv.URL)

	// Create a server so we can seed library items
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Sync a library item with external IDs
	ctx := context.Background()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie-1",
		MediaType: models.MediaTypeMovie,
		Title:     "Test Movie",
		Year:      2024,
		AddedAt:   time.Now().UTC(),
		TMDBID:    "27205",
		TVDBID:    "",
		IMDBID:    "tt1375666",
	}}
	if _, _, err := s.SyncLibraryItems(ctx, srv.ID, "lib1", items); err != nil {
		t.Fatal(err)
	}

	// Create a maintenance rule + candidate
	rule := &models.MaintenanceRuleInput{
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
		Name:          "test rule",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
		Enabled:       true,
	}
	created, err := s.CreateMaintenanceRule(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}

	// Get the library item ID
	libItems, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(libItems) == 0 {
		t.Fatal("no library items found")
	}
	libItem := libItems[0]

	if err := s.UpsertMaintenanceCandidate(ctx, created.ID, libItem.ID, "test reason"); err != nil {
		t.Fatal(err)
	}

	// Fetch candidates back and verify external IDs survived the round trip
	candidates, err := s.ListCandidatesForRule(ctx, created.ID, 1, 100, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates.Items) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates.Items))
	}

	c := candidates.Items[0]
	if c.Item == nil {
		t.Fatal("candidate item is nil")
	}
	if c.Item.TMDBID != "27205" {
		t.Errorf("expected TMDBID 27205, got %q", c.Item.TMDBID)
	}
	if c.Item.IMDBID != "tt1375666" {
		t.Errorf("expected IMDBID tt1375666, got %q", c.Item.IMDBID)
	}

	// Now cascade delete using the candidate's item
	cd := NewCascadeDeleter(s)
	results := cd.DeleteExternalReferences(ctx, c.Item)

	if !radarrDeleted.Load() {
		t.Error("expected Radarr movie to be deleted via DB round-trip")
	}
	radarrResult := findResult(results, "radarr")
	if radarrResult == nil || !radarrResult.Success {
		t.Errorf("expected radarr success, got %+v", radarrResult)
	}
}

func TestUpdateSonarrMonitoring(t *testing.T) {
	var seasonPassBody []byte
	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/series" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]any{{"id": 77}})
		case r.URL.Path == "/api/v3/seasonpass" && r.Method == http.MethodPost:
			var err error
			seasonPassBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sonarrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "sonarr", sonarrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Long Show",
		MediaType: models.MediaTypeTV,
		TVDBID:    "67890",
	}

	result := cd.UpdateSonarrMonitoring(context.Background(), item)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Service != "sonarr" {
		t.Errorf("expected service sonarr, got %s", result.Service)
	}

	if seasonPassBody == nil {
		t.Fatal("expected seasonpass POST request to be made")
	}

	var body struct {
		Series            []struct{ ID int `json:"id"` } `json:"series"`
		MonitoringOptions struct{ Monitor string `json:"monitor"` } `json:"monitoringOptions"`
	}
	if err := json.Unmarshal(seasonPassBody, &body); err != nil {
		t.Fatalf("unmarshal seasonpass body: %v", err)
	}

	if len(body.Series) != 1 || body.Series[0].ID != 77 {
		t.Errorf("expected series [{id:77}], got %v", body.Series)
	}
	if body.MonitoringOptions.Monitor != "future" {
		t.Errorf("expected monitor=future, got %q", body.MonitoringOptions.Monitor)
	}
}

func TestUpdateSonarrMonitoringNotFoundInSonarr(t *testing.T) {
	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/series" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]any{}) // empty result
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sonarrSrv.Close()

	s := newTestStoreWithMigrations(t)
	configureIntegration(t, s, "sonarr", sonarrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "Unknown Show",
		MediaType: models.MediaTypeTV,
		TVDBID:    "99999",
	}

	result := cd.UpdateSonarrMonitoring(context.Background(), item)

	if result.Success {
		t.Error("expected no success for series not found in Sonarr")
	}
	if result.Error != "" {
		t.Errorf("expected no error for not-found, got %s", result.Error)
	}
}

func TestUpdateSonarrMonitoringNoTVDBID(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not make any requests when TVDBID is empty")
	}))
	defer sonarrSrv.Close()
	configureIntegration(t, s, "sonarr", sonarrSrv.URL)

	cd := NewCascadeDeleter(s)
	item := &models.LibraryItemCache{
		Title:     "No TVDB Show",
		MediaType: models.MediaTypeTV,
		TVDBID:    "",
	}

	result := cd.UpdateSonarrMonitoring(context.Background(), item)

	if result.Success {
		t.Error("expected no success for item without TVDB ID")
	}
}

func findResult(results []CascadeResult, service string) *CascadeResult {
	for _, r := range results {
		if r.Service == service {
			return &r
		}
	}
	return nil
}
