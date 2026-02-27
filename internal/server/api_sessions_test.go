package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestTerminateSession_NoPoller(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate",
		strings.NewReader(`{"server_id":1,"session_id":"abc"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (no poller), got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminateSession_InvalidJSON(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	setupTestPoller(t, srv.Unwrap(), st)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate",
		strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminateSession_MissingFields(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	setupTestPoller(t, srv.Unwrap(), st)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate",
		strings.NewReader(`{"server_id":0,"session_id":""}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminateSession_ServerNotFound(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	setupTestPoller(t, srv.Unwrap(), st)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate",
		strings.NewReader(`{"server_id":999,"session_id":"abc"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminateSession_ViewerDenied(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	setupTestPoller(t, srv.Unwrap(), st)

	viewerToken := createViewerSession(t, st, "viewer1")

	body := `{"server_id":1,"session_id":"abc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "streammon_session", Value: viewerToken})
	w := httptest.NewRecorder()
	srv.Unwrap().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTerminateSession_DefaultMessage(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestPlex", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{}
	mock.srvType = models.ServerTypePlex
	p.AddServer(s.ID, mock)

	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid"}`, s.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.terminatedMsg != defaultTerminateMessage {
		t.Errorf("expected default message %q, got %q", defaultTerminateMessage, mock.terminatedMsg)
	}
}

type mockTerminateServer struct {
	mockLibraryServer
	terminatedID  string
	terminatedMsg string
	terminateErr  error
}

func (m *mockTerminateServer) TerminateSession(ctx context.Context, sessionID string, message string) error {
	m.terminatedID = sessionID
	m.terminatedMsg = message
	return m.terminateErr
}

func TestTerminateSession_Success(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestPlex", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{}
	mock.srvType = models.ServerTypePlex
	p.AddServer(s.ID, mock)

	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess-key","plex_session_uuid":"uuid-123","message":"go away"}`, s.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.terminatedID != "uuid-123" {
		t.Errorf("expected terminate ID %q, got %q", "uuid-123", mock.terminatedID)
	}
	if mock.terminatedMsg != "go away" {
		t.Errorf("expected message %q, got %q", "go away", mock.terminatedMsg)
	}
}

func TestTerminateSession_PlexFallbackToSessionID(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestPlex", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{}
	mock.srvType = models.ServerTypePlex
	p.AddServer(s.ID, mock)

	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess-key","message":"bye"}`, s.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.terminatedID != "sess-key" {
		t.Errorf("expected fallback to session_id %q, got %q", "sess-key", mock.terminatedID)
	}
}

func TestTerminateSession_FailurePlexPass(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestPlex", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{
		terminateErr: fmt.Errorf("plex terminate: %w", models.ErrPlexPassRequired),
	}
	mock.srvType = models.ServerTypePlex
	p.AddServer(s.ID, mock)

	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid"}`, s.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Plex Pass") {
		t.Errorf("expected Plex Pass message in body, got: %s", w.Body.String())
	}
}

func TestTerminateSession_GenericFailure(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestEmby", Type: models.ServerTypeEmby,
		URL: "http://emby", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{
		terminateErr: fmt.Errorf("emby post: status 500"),
	}
	mock.srvType = models.ServerTypeEmby
	p.AddServer(s.ID, mock)

	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess"}`, s.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to terminate") {
		t.Errorf("expected generic failure message, got: %s", w.Body.String())
	}
}

func TestTerminateSession_MessageTruncation(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := setupTestPoller(t, srv.Unwrap(), st)

	s := &models.Server{
		Name: "TestPlex", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	}
	st.CreateServer(s)
	mock := &mockTerminateServer{}
	mock.srvType = models.ServerTypePlex
	p.AddServer(s.ID, mock)

	longMsg := strings.Repeat("a", 600)
	body := fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid","message":"%s"}`, s.ID, longMsg)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(mock.terminatedMsg) != 500 {
		t.Errorf("expected message truncated to 500, got %d", len(mock.terminatedMsg))
	}
}
