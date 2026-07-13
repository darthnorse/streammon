package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

// A brand-new media-server user who is streaming for the first time has no
// finalized watch_history rows and is not in the users (login/synced) table.
// The user-detail page must still load rather than 404 (regression: "Failed to
// load user statistics").

func TestGetUserStats_MediaUserBelowThreshold_Returns200(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	srv := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	if err := st.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	// Full-length movie watched only briefly: excluded by the min-play filter, so
	// SessionCount==0, yet the user genuinely exists in watch_history.
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "newbie", MediaType: models.MediaTypeMovie,
		Title: "Movie", DurationMs: 7200000, WatchedMs: 1000,
		StartedAt: now, StoppedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/newbie/stats", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetUserStats_StreamingUserNoHistory_Returns200(t *testing.T) {
	ts, _ := newTestServerWrapped(t)
	ts.SetPollerForTest(&fakePoller{sessions: []models.ActiveStream{{UserName: "streamer"}}})

	req := httptest.NewRequest(http.MethodGet, "/api/users/streamer/stats", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetUserStats_UnknownUser_Returns404(t *testing.T) {
	ts, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/ghost/stats", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetUser_MediaUserNotInUsersTable_Returns200(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	srv := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k", Enabled: true}
	if err := st.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "mediaonly", MediaType: models.MediaTypeMovie,
		Title: "Movie", StartedAt: now, StoppedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/mediaonly", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var u models.User
	if err := json.NewDecoder(w.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Name != "mediaonly" {
		t.Errorf("expected name mediaonly, got %q", u.Name)
	}
	if u.Role != models.RoleViewer {
		t.Errorf("expected viewer role, got %q", u.Role)
	}
}

func TestGetUser_StreamingUserNoHistory_Returns200(t *testing.T) {
	ts, _ := newTestServerWrapped(t)
	ts.SetPollerForTest(&fakePoller{sessions: []models.ActiveStream{{UserName: "livestreamer"}}})

	req := httptest.NewRequest(http.MethodGet, "/api/users/livestreamer", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetUser_UnknownUser_Returns404(t *testing.T) {
	ts, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/ghost", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}
