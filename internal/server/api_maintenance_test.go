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

	body := w.Body.String()
	if !strings.Contains(body, "ID,Title,Media Type") {
		t.Errorf("CSV should contain header row, got: %s", body)
	}
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

// setupDeleteCandidateTest creates server, library item, rule, and candidate for delete tests.
// Returns the server ID for poller setup.
func setupDeleteCandidateTest(t *testing.T, s interface {
	CreateServer(*models.Server) error
	UpsertLibraryItems(context.Context, []models.LibraryItemCache) (int, error)
	CreateMaintenanceRule(context.Context, *models.MaintenanceRuleInput) (*models.MaintenanceRule, error)
	UpsertMaintenanceCandidate(context.Context, int64, int64, string) error
}, itemID string) int64 {
	t.Helper()
	ctx := context.Background()

	server := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(server); err != nil {
		t.Fatal(err)
	}

	items := []models.LibraryItemCache{
		{ServerID: server.ID, LibraryID: "lib1", ItemID: itemID, MediaType: models.MediaTypeMovie, Title: "Test Movie", Year: 2020, AddedAt: time.Now().UTC()},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
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

	if err := s.UpsertMaintenanceCandidate(ctx, rule.ID, 1, "test reason"); err != nil {
		t.Fatal(err)
	}

	return server.ID
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
	setupDeleteCandidateTest(t, s, "item1")

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
	serverID := setupDeleteCandidateTest(t, s, "item123")

	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{}
	p.AddServer(serverID, mock)

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	if len(mock.deleted) != 1 || mock.deleted[0] != "item123" {
		t.Errorf("expected delete of item123, got %v", mock.deleted)
	}

	_, err := s.GetMaintenanceCandidate(ctx, 1)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected candidate to be deleted, got err: %v", err)
	}
}

func TestDeleteCandidateServerFailureAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	serverID := setupDeleteCandidateTest(t, s, "item456")

	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{deleteErr: errors.New("media server unavailable")}
	p.AddServer(serverID, mock)

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/candidates/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	_, err := s.GetMaintenanceCandidate(ctx, 1)
	if err != nil {
		t.Errorf("expected candidate to still exist, got err: %v", err)
	}
}

func TestCreateExclusionsAPI(t *testing.T) {
	srv, s := newTestServer(t)
	serverID := setupDeleteCandidateTest(t, s, "item1")

	body := `{"library_item_ids":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/rules/1/exclusions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["excluded"].(float64) != 1 {
		t.Errorf("excluded = %v, want 1", resp["excluded"])
	}

	count, _ := s.CountExclusionsForRule(context.Background(), 1)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	_ = serverID
}

func TestListExclusionsAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	setupDeleteCandidateTest(t, s, "item1")

	if _, err := s.CreateExclusions(ctx, 1, []int64{1}, "test@test.com"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/exclusions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", resp["total"])
	}
}

func TestDeleteExclusionAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	setupDeleteCandidateTest(t, s, "item1")

	if _, err := s.CreateExclusions(ctx, 1, []int64{1}, "test@test.com"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/maintenance/rules/1/exclusions/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	count, _ := s.CountExclusionsForRule(ctx, 1)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestBulkRemoveExclusionsAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	setupDeleteCandidateTest(t, s, "item1")

	if _, err := s.CreateExclusions(ctx, 1, []int64{1}, "test@test.com"); err != nil {
		t.Fatal(err)
	}

	body := `{"library_item_ids":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/rules/1/exclusions/bulk-remove", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	count, _ := s.CountExclusionsForRule(ctx, 1)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestBulkDeleteCandidatesAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	serverID := setupDeleteCandidateTest(t, s, "item1")

	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{}
	p.AddServer(serverID, mock)

	body := `{"candidate_ids":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/candidates/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["deleted"].(float64) != 1 {
		t.Errorf("deleted = %v, want 1", resp["deleted"])
	}
	if resp["failed"].(float64) != 0 {
		t.Errorf("failed = %v, want 0", resp["failed"])
	}

	_, err := s.GetMaintenanceCandidate(ctx, 1)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected candidate to be deleted")
	}
}

func TestBulkDeleteCandidatesPartialFailureAPI(t *testing.T) {
	srv, s := newTestServer(t)
	serverID := setupDeleteCandidateTest(t, s, "item1")

	p := poller.New(s, time.Hour)
	srv.poller = p
	pCtx, cancel := context.WithCancel(context.Background())
	p.Start(pCtx)
	t.Cleanup(func() {
		cancel()
		p.Stop()
	})

	mock := &mockDeleteServer{deleteErr: errors.New("server error")}
	p.AddServer(serverID, mock)

	body := `{"candidate_ids":[1, 99999]}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/candidates/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["deleted"].(float64) != 0 {
		t.Errorf("deleted = %v, want 0", resp["deleted"])
	}
	if resp["failed"].(float64) != 2 {
		t.Errorf("failed = %v, want 2", resp["failed"])
	}
	errs := resp["errors"].([]any)
	if len(errs) != 2 {
		t.Errorf("errors count = %d, want 2", len(errs))
	}
}

func TestExcludedCandidatesFilteredFromListAPI(t *testing.T) {
	srv, s := newTestServer(t)
	ctx := context.Background()
	setupDeleteCandidateTest(t, s, "item1")

	// List candidates - should have 1
	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["total"].(float64) != 1 {
		t.Fatalf("expected 1 candidate before exclusion, got %v", resp["total"])
	}

	// Exclude the item
	if _, err := s.CreateExclusions(ctx, 1, []int64{1}, "test@test.com"); err != nil {
		t.Fatal(err)
	}

	// List candidates again - should have 0
	req = httptest.NewRequest(http.MethodGet, "/api/maintenance/rules/1/candidates", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp["total"].(float64) != 0 {
		t.Errorf("expected 0 candidates after exclusion, got %v", resp["total"])
	}
}
