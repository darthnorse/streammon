package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestListUsersAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.GetOrCreateUser("alice")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var users []models.User
	json.NewDecoder(w.Body).Decode(&users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestGetUserAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.GetOrCreateUser("alice")

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)
	if user.Name != "alice" {
		t.Fatalf("expected alice, got %s", user.Name)
	}
}

func TestGetUserNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/nobody", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetUserLocationsAPI(t *testing.T) {
	srv, st := newTestServer(t)

	st.CreateServer(&models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"})

	now := time.Now()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: 1, UserName: "alice", MediaType: "movie", Title: "A",
		IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var locs []models.GeoResult
	json.NewDecoder(w.Body).Decode(&locs)
	if locs == nil {
		t.Fatal("expected [], got null")
	}
}

func TestGetUserLocationsNoHistoryAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/nobody/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserLocationsCachedAPI(t *testing.T) {
	srv, st := newTestServer(t)

	st.CreateServer(&models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"})

	now := time.Now()
	st.InsertHistory(&models.WatchHistoryEntry{
		ServerID: 1, UserName: "alice", MediaType: "movie", Title: "A",
		IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now,
	})

	st.SetCachedGeo(&models.GeoResult{
		IP: "8.8.8.8", Lat: 37.386, Lng: -122.084, City: "Mountain View", Country: "US",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/alice/locations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var locs []models.GeoResult
	json.NewDecoder(w.Body).Decode(&locs)
	if len(locs) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locs))
	}
	if locs[0].City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", locs[0].City)
	}
}
