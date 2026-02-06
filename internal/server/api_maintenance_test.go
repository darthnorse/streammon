package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/poller"
)

func TestGetCriterionTypesAPI(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/criterion-types", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	types, ok := resp["types"]
	if !ok {
		t.Fatal("expected 'types' key in response")
	}
	if len(types.([]any)) == 0 {
		t.Error("expected at least one criterion type")
	}
}

func TestCreateMaintenanceRuleAPI(t *testing.T) {
	srv, s := newTestServer(t)

	// Create server first
	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	body := `{"server_id":1,"library_id":"lib1","name":"Test Rule","criterion_type":"unwatched_movie","parameters":{},"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var rule models.MaintenanceRule
	if err := json.NewDecoder(w.Body).Decode(&rule); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rule.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if rule.Name != "Test Rule" {
		t.Errorf("name = %q, want %q", rule.Name, "Test Rule")
	}
}

func TestCreateMaintenanceRuleAPIValidation(t *testing.T) {
	srv, s := newTestServer(t)

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"server_id":1,"library_id":"lib1","criterion_type":"unwatched_movie"}`},
		{"invalid criterion", `{"server_id":1,"library_id":"lib1","name":"X","criterion_type":"invalid"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/maintenance/rules", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 or 500, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGetMaintenanceRuleAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got models.MaintenanceRule
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != rule.ID {
		t.Errorf("ID = %d, want %d", got.ID, rule.ID)
	}
}

func TestGetMaintenanceRuleNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/99999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListMaintenanceRulesAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	// Create a rule
	_, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	rules := resp["rules"].([]any)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestDeleteMaintenanceRuleAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/rules/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

func TestExportCandidatesCSVAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates/export?format=csv", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition should contain 'attachment': %s", cd)
	}

	// Body should contain CSV header
	body := w.Body.String()
	if !strings.Contains(body, "ID,Title,Media Type") {
		t.Errorf("CSV should contain header row, got: %s", body)
	}

	_ = rule // avoid unused
}

func TestExportCandidatesJSONAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates/export?format=json", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["rule_id"] == nil {
		t.Error("expected rule_id in response")
	}
	if resp["candidates"] == nil {
		t.Error("expected candidates in response")
	}
	if resp["total"] == nil {
		t.Error("expected total in response")
	}
}

func TestExportCandidatesInvalidFormatAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates/export?format=xml", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCandidatesAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["items"] == nil {
		t.Error("expected 'items' in response")
	}
	if resp["total"] == nil {
		t.Error("expected 'total' in response")
	}
}

// mockDeleteServer implements media.MediaServer for delete candidate tests
type mockDeleteServer struct {
	deleteErr error
	deleted   []string
}

func (m *mockDeleteServer) Name() string                                            { return "mock" }
func (m *mockDeleteServer) Type() models.ServerType                                 { return models.ServerTypePlex }
func (m *mockDeleteServer) TestConnection(ctx context.Context) error                { return nil }
func (m *mockDeleteServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockDeleteServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockDeleteServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	return nil, nil
}
func (m *mockDeleteServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockDeleteServer) ServerID() int64 { return 1 }
func (m *mockDeleteServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockDeleteServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockDeleteServer) DeleteItem(ctx context.Context, itemID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, itemID)
	return nil
}

func TestDeleteCandidateNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/99999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCandidateInvalidIDAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/invalid", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCandidateServerNotFoundAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	// Create server in DB
	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	// Create library item
	items := []models.LibraryItemCache{
		{ServerID: server.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "Test Movie", Year: 2020, AddedAt: time.Now().UTC()},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Create rule
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create candidate
	if err := s.UpsertMaintenanceCandidate(ctx, rule.ID, 1, "test reason"); err != nil {
		t.Fatal(err)
	}

	// No poller set up, so server won't be found
	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (server not found), got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCandidateSuccessAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	// Create server in DB
	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	// Create library item
	items := []models.LibraryItemCache{
		{ServerID: server.ID, LibraryID: "lib1", ItemID: "item123", MediaType: models.MediaTypeMovie, Title: "Test Movie", Year: 2020, AddedAt: time.Now().UTC()},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Create rule
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create candidate
	if err := s.UpsertMaintenanceCandidate(ctx, rule.ID, 1, "test reason"); err != nil {
		t.Fatal(err)
	}

	// Set up poller with mock server
	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{}
	p.AddServer(server.ID, mock)

	// Delete the candidate
	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify item was deleted from mock
	if len(mock.deleted) != 1 || mock.deleted[0] != "item123" {
		t.Errorf("expected delete of item123, got %v", mock.deleted)
	}

	// Verify candidate is gone
	_, err = s.GetMaintenanceCandidate(ctx, 1)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected candidate to be deleted, got err: %v", err)
	}
}

func TestDeleteCandidateServerFailureAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()

	// Create server in DB
	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	// Create library item
	items := []models.LibraryItemCache{
		{ServerID: server.ID, LibraryID: "lib1", ItemID: "item456", MediaType: models.MediaTypeMovie, Title: "Test Movie", Year: 2020, AddedAt: time.Now().UTC()},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Create rule
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      server.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create candidate
	if err := s.UpsertMaintenanceCandidate(ctx, rule.ID, 1, "test reason"); err != nil {
		t.Fatal(err)
	}

	// Set up poller with mock server that fails
	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{deleteErr: errors.New("media server unavailable")}
	p.AddServer(server.ID, mock)

	// Try to delete the candidate
	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	// Verify candidate is still there (not deleted on server failure)
	_, err = s.GetMaintenanceCandidate(ctx, 1)
	if err != nil {
		t.Errorf("expected candidate to still exist, got err: %v", err)
	}
}
