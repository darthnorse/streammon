package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestCreateServerAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	// Plex servers require machine_id
	body := `{"name":"Plex","type":"plex","url":"http://plex:32400","api_key":"abc","machine_id":"abc123","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/servers", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var s models.Server
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if s.ID == 0 {
		t.Fatal("expected ID")
	}
	if s.Name != "Plex" {
		t.Fatalf("expected Plex, got %s", s.Name)
	}
	if s.APIKey != "" {
		t.Fatal("api_key should not be in JSON response")
	}
	if s.MachineID != "abc123" {
		t.Fatalf("expected machine_id abc123, got %s", s.MachineID)
	}
}

func TestCreateServerValidationAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":"","type":"plex","url":"http://x","api_key":"k","machine_id":"m"}`},
		{"bad type", `{"name":"X","type":"invalid","url":"http://x","api_key":"k"}`},
		{"empty url", `{"name":"X","type":"plex","url":"","api_key":"k","machine_id":"m"}`},
		{"empty api_key", `{"name":"X","type":"plex","url":"http://x","api_key":"","machine_id":"m"}`},
		{"plex without machine_id", `{"name":"X","type":"plex","url":"http://x","api_key":"k"}`},
		{"invalid json", `{bad`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/servers", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestListServersAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{Name: "A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var servers []models.Server
	if err := json.NewDecoder(w.Body).Decode(&servers); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
}

func TestListServersEmptyAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Body.String() == "null\n" {
		t.Fatal("expected [], got null")
	}
}

func TestGetServerAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/servers/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateServerAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{Name: "Old", Type: models.ServerTypePlex, URL: "http://old", APIKey: "k", MachineID: "machine123", Enabled: true})

	body := `{"name":"New","type":"plex","url":"http://new","api_key":"k2","machine_id":"machine123","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated models.Server
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Name != "New" {
		t.Fatalf("expected New, got %s", updated.Name)
	}
}

func TestUpdateServerRequiresMachineIDForPlex(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	// Create a legacy Plex server without machine_id
	st.CreateServer(&models.Server{Name: "Legacy", Type: models.ServerTypePlex, URL: "http://legacy", APIKey: "k", Enabled: true})

	// Try to update without providing machine_id - should fail
	body := `{"name":"Updated","type":"plex","url":"http://legacy","api_key":"k","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing machine_id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateServerPreservesAPIKeyWhenEmpty(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{Name: "Old", Type: models.ServerTypePlex, URL: "http://old", APIKey: "secret", MachineID: "m123", Enabled: true})

	// machine_id provided, api_key empty - should preserve api_key
	body := `{"name":"New","type":"plex","url":"http://new","api_key":"","machine_id":"m123","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := st.GetServer(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "secret" {
		t.Fatalf("expected api_key preserved as 'secret', got %q", got.APIKey)
	}
	if got.Name != "New" {
		t.Fatalf("expected name 'New', got %s", got.Name)
	}
}

func TestUpdateServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"name":"X","type":"plex","url":"http://x","api_key":"k","machine_id":"m123","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/999", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/servers/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestServerAdHocValidationAPI(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"name":"","type":"plex","url":"http://x","api_key":"k"}`
	req := httptest.NewRequest(http.MethodPost, "/api/servers/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestServerAdHocInvalidJSON(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/servers/test", strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteServerAPI(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{Name: "X", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k"})

	req := httptest.NewRequest(http.MethodDelete, "/api/servers/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestCreateServerWithMachineID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	body := `{"name":"Plex","type":"plex","url":"http://plex:32400","api_key":"abc","machine_id":"abc123def456","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/servers", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var s models.Server
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if s.MachineID != "abc123def456" {
		t.Fatalf("expected machine_id abc123def456, got %s", s.MachineID)
	}

	// Verify it's stored in the database
	got, err := st.GetServer(s.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.MachineID != "abc123def456" {
		t.Fatalf("expected stored machine_id abc123def456, got %s", got.MachineID)
	}
}

func TestUpdateServerPreservesMachineID(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{
		Name:      "Plex",
		Type:      models.ServerTypePlex,
		URL:       "http://plex",
		APIKey:    "secret",
		MachineID: "original-machine-id",
		Enabled:   true,
	})

	// Update without machine_id in payload - should preserve the original
	body := `{"name":"New Plex","type":"plex","url":"http://new-plex","api_key":"","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := st.GetServer(1)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.MachineID != "original-machine-id" {
		t.Fatalf("expected machine_id to be preserved as original-machine-id, got %s", got.MachineID)
	}
	if got.Name != "New Plex" {
		t.Fatalf("expected name 'New Plex', got %s", got.Name)
	}
}

func TestUpdateServerWithNewMachineID(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateServer(&models.Server{
		Name:      "Plex",
		Type:      models.ServerTypePlex,
		URL:       "http://plex",
		APIKey:    "secret",
		MachineID: "old-machine-id",
		Enabled:   true,
	})

	// Update with new machine_id
	body := `{"name":"Plex","type":"plex","url":"http://plex","api_key":"","machine_id":"new-machine-id","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := st.GetServer(1)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.MachineID != "new-machine-id" {
		t.Fatalf("expected machine_id to be updated to new-machine-id, got %s", got.MachineID)
	}
}

func TestUpdateServerClearsMaintenanceDataOnURLChange(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	// Create a Plex server
	st.CreateServer(&models.Server{
		Name:      "Plex",
		Type:      models.ServerTypePlex,
		URL:       "http://old-plex:32400",
		APIKey:    "secret",
		MachineID: "machine123",
		Enabled:   true,
	})

	// Add some library items to simulate cached data
	ctx := context.Background()
	_, err := st.UpsertLibraryItems(ctx, []models.LibraryItemCache{
		{ServerID: 1, LibraryID: "1", ItemID: "item1", MediaType: "movie", Title: "Movie 1"},
		{ServerID: 1, LibraryID: "1", ItemID: "item2", MediaType: "movie", Title: "Movie 2"},
	})
	if err != nil {
		t.Fatalf("upsert library items: %v", err)
	}

	// Verify items exist
	count, err := st.CountLibraryItems(ctx, 1, "1")
	if err != nil {
		t.Fatalf("count items: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 items before update, got %d", count)
	}

	// Update server with new URL - should clear maintenance data
	body := `{"name":"Plex","type":"plex","url":"http://new-plex:32400","api_key":"","machine_id":"machine123","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify items were deleted
	count, err = st.CountLibraryItems(ctx, 1, "1")
	if err != nil {
		t.Fatalf("count items after: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 items after URL change, got %d", count)
	}
}

func TestUpdateServerPreservesMaintenanceDataOnSameURL(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	// Create a Plex server
	st.CreateServer(&models.Server{
		Name:      "Plex",
		Type:      models.ServerTypePlex,
		URL:       "http://plex:32400",
		APIKey:    "secret",
		MachineID: "machine123",
		Enabled:   true,
	})

	// Add some library items to simulate cached data
	ctx := context.Background()
	_, err := st.UpsertLibraryItems(ctx, []models.LibraryItemCache{
		{ServerID: 1, LibraryID: "1", ItemID: "item1", MediaType: "movie", Title: "Movie 1"},
	})
	if err != nil {
		t.Fatalf("upsert library items: %v", err)
	}

	// Update server with same URL, just change name
	body := `{"name":"My Plex","type":"plex","url":"http://plex:32400","api_key":"","machine_id":"machine123","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify items still exist
	count, err := st.CountLibraryItems(ctx, 1, "1")
	if err != nil {
		t.Fatalf("count items after: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 item preserved when URL unchanged, got %d", count)
	}
}
