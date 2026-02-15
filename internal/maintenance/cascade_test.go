package maintenance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

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

	var overseerrDeleted atomic.Bool
	overseerrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/movie/27205" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"id": 27205,
				"mediaInfo": map[string]any{
					"requests": []map[string]any{{"id": 10}},
				},
			})
		case r.URL.Path == "/api/v1/request/10" && r.Method == http.MethodDelete:
			overseerrDeleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
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
	if !overseerrDeleted.Load() {
		t.Error("expected Overseerr request to be deleted")
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

	var overseerrDeleted atomic.Bool
	overseerrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/tv/12345" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"id": 12345,
				"mediaInfo": map[string]any{
					"requests": []map[string]any{{"id": 20}},
				},
			})
		case r.URL.Path == "/api/v1/request/20" && r.Method == http.MethodDelete:
			overseerrDeleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
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
	if !overseerrDeleted.Load() {
		t.Error("expected Overseerr request to be deleted")
	}

	sonarrResult := findResult(results, "sonarr")
	if sonarrResult == nil || !sonarrResult.Success {
		t.Errorf("expected sonarr success, got %+v", sonarrResult)
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
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		Name:          "test rule",
		CriterionType: models.CriterionUnwatchedMovie,
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
	candidates, err := s.ListCandidatesForRule(ctx, created.ID, 1, 100, "")
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

func findResult(results []CascadeResult, service string) *CascadeResult {
	for _, r := range results {
		if r.Service == service {
			return &r
		}
	}
	return nil
}
