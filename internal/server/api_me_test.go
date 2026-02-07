package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestHandleMe_ReturnsUser(t *testing.T) {
	srv, _ := newTestServerWrapped(t)
	user := &models.User{ID: 1, Name: "alice", Email: "alice@example.com", Role: models.RoleAdmin}

	req := httptest.NewRequest("GET", "/api/me", nil)
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleMe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got models.User
	json.NewDecoder(w.Body).Decode(&got)
	if got.Name != "alice" {
		t.Errorf("got name %q, want alice", got.Name)
	}
}

func TestHandleMe_NoUser_Returns401(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest("GET", "/api/me", nil)
	w := httptest.NewRecorder()
	srv.handleMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
