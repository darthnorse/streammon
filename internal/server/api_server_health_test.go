package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func getServerHealth(t *testing.T, ts *testServer, cookie string) (*httptest.ResponseRecorder, serverHealthResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/server-health", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)
	var resp serverHealthResponse
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return w, resp
}

func TestServerHealth_NoPoller_ReturnsEmptyList(t *testing.T) {
	ts, _ := newTestServerWrapped(t)
	w, _ := getServerHealth(t, ts, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "{\"failing\":[]}\n" {
		t.Errorf("body = %q, want empty failing array", got)
	}
}

func TestServerHealth_ReturnsFailingServers(t *testing.T) {
	ts, _ := newTestServerWrapped(t)
	since := time.Date(2026, 10, 29, 3, 37, 36, 0, time.UTC)
	ts.Server.SetPollerForTest(&fakePoller{failing: []models.ServerPollFailure{
		{ServerID: 3, ServerName: "Jellyfin", Error: "jellyfin returned status 401", FailingSince: since},
	}})

	w, resp := getServerHealth(t, ts, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(resp.Failing) != 1 {
		t.Fatalf("expected 1 failing server, got %+v", resp.Failing)
	}
	f := resp.Failing[0]
	if f.ServerID != 3 || f.ServerName != "Jellyfin" || f.Error != "jellyfin returned status 401" || !f.FailingSince.Equal(since) {
		t.Errorf("unexpected entry: %+v", f)
	}
}

func TestServerHealth_ForbiddenForViewer(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	ts.Server.SetPollerForTest(&fakePoller{failing: []models.ServerPollFailure{}})
	token := createViewerSession(t, st, "guest")

	w, _ := getServerHealth(t, ts, token)
	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", w.Code)
	}
}
