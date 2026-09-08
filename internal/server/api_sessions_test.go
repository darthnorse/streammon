package server

import (
	"context"
	"errors"
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

// newTerminateFixture wires a server whose poller resolves server IDs to mock.
// The handler only ever asks the poller for the media server, so a fake keeps
// these tests free of a live poll loop.
func newTerminateFixture(t *testing.T, srvType models.ServerType, terminateErr error) (*testServer, *fakePoller, *mockTerminateServer, int64) {
	t.Helper()
	srv, st := newTestServerWrapped(t)

	mock := &mockTerminateServer{terminateErr: terminateErr}
	mock.srvType = srvType
	fake := &fakePoller{ms: mock}
	srv.Unwrap().SetPollerForTest(fake)

	name, url := "TestPlex", "http://plex"
	if srvType != models.ServerTypePlex {
		name, url = "TestEmby", "http://emby"
	}
	s := &models.Server{Name: name, Type: srvType, URL: url, APIKey: "k", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv, fake, mock, s.ID
}

func postTerminate(t *testing.T, srv *testServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/terminate", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestTerminateSession_DefaultMessage(t *testing.T) {
	srv, _, mock, id := newTerminateFixture(t, models.ServerTypePlex, nil)

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid"}`, id))

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
	srv, _, mock, id := newTerminateFixture(t, models.ServerTypePlex, nil)

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess-key","plex_session_uuid":"uuid-123","message":"go away"}`, id))

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
	srv, _, mock, id := newTerminateFixture(t, models.ServerTypePlex, nil)

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess-key","message":"bye"}`, id))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mock.terminatedID != "sess-key" {
		t.Errorf("expected fallback to session_id %q, got %q", "sess-key", mock.terminatedID)
	}
}

func TestTerminateSession_FailurePlexPass(t *testing.T) {
	srv, _, _, id := newTerminateFixture(t, models.ServerTypePlex,
		fmt.Errorf("plex terminate: %w", models.ErrPlexPassRequired))

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid"}`, id))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Plex Pass") {
		t.Errorf("expected Plex Pass message in body, got: %s", w.Body.String())
	}
}

func TestTerminateSession_GenericFailure(t *testing.T) {
	srv, _, _, id := newTerminateFixture(t, models.ServerTypeEmby,
		fmt.Errorf("emby post: status 500"))

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess"}`, id))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to terminate") {
		t.Errorf("expected generic failure message, got: %s", w.Body.String())
	}
}

func TestTerminateSession_MessageTruncation(t *testing.T) {
	srv, _, mock, id := newTerminateFixture(t, models.ServerTypePlex, nil)

	longMsg := strings.Repeat("a", 600)
	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess","plex_session_uuid":"uuid","message":"%s"}`, id, longMsg))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(mock.terminatedMsg) != 500 {
		t.Errorf("expected message truncated to 500, got %d", len(mock.terminatedMsg))
	}
}

func TestTerminateSession_TriggersPoll(t *testing.T) {
	srv, fake, _, id := newTerminateFixture(t, models.ServerTypePlex, nil)

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess"}`, id))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := fake.triggerCount(); got != 1 {
		t.Fatalf("terminate triggered %d polls, want 1", got)
	}
}

func TestTerminateSession_FailureDoesNotTriggerPoll(t *testing.T) {
	srv, fake, _, id := newTerminateFixture(t, models.ServerTypePlex, errors.New("nope"))

	w := postTerminate(t, srv, fmt.Sprintf(`{"server_id":%d,"session_id":"sess"}`, id))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	if got := fake.triggerCount(); got != 0 {
		t.Fatalf("failed terminate triggered %d polls, want 0", got)
	}
}
