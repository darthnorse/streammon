package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
