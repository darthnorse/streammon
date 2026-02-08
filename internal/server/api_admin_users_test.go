package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
	"streammon/internal/store"
)

func TestAdminListUsers(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var users []store.AdminUser
	json.NewDecoder(w.Body).Decode(&users)
	// test-admin + alice
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestAdminMergeUsers(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	st.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := st.GetUser("alice")
	bob, _ := st.GetUser("bob")

	body := fmt.Sprintf(`{"keep_id": %d, "delete_id": %d}`, alice.ID, bob.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/merge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify bob is deleted
	_, err := st.GetUser("bob")
	if err == nil {
		t.Error("bob should be deleted after merge")
	}
}

func TestAdminMergeUsers_SameUser(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := st.GetUser("alice")

	body := fmt.Sprintf(`{"keep_id": %d, "delete_id": %d}`, alice.ID, alice.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/merge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for same user merge, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminMergeUsers_DeleteSelf(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := st.GetUser("alice")

	// test-admin is ID 1 (created by test setup)
	body := fmt.Sprintf(`{"keep_id": %d, "delete_id": 1}`, alice.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/merge", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deleting self, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminUnlinkUser(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	// Create user with password AND provider
	st.CreateLocalUser("alice", "alice@example.com", "hash123", models.RoleViewer)
	alice, _ := st.GetUser("alice")
	st.LinkProviderAccount(alice.ID, "plex", "plex-id-123")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/users/%d/unlink", alice.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify provider is cleared
	user, _ := st.GetAdminUserByID(alice.ID)
	if user.Provider != "" {
		t.Errorf("expected empty provider, got %s", user.Provider)
	}
}

func TestAdminUnlinkUser_NoPassword(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	// Create OAuth-only user via GetOrLinkUser (no password)
	alice, err := st.GetOrLinkUser("alice@example.com", []string{}, "alice", "plex", "12345", "")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/users/%d/unlink", alice.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unlink without password, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminUnlinkUser_Self(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	// test-admin is ID 1, try to unlink self
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/1/unlink", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unlinking self, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminDeleteUser(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := st.GetUser("alice")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", alice.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify alice is deleted
	_, err := st.GetUser("alice")
	if err == nil {
		t.Error("alice should be deleted")
	}
}

func TestAdminDeleteUser_LastAdmin(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	// test-admin is the only admin, should not be deletable
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deleting last admin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminUpdateUserRole(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := st.GetUser("alice")

	body := `{"role": "admin"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%d/role", alice.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify role updated
	updated, _ := st.GetUser("alice")
	if updated.Role != models.RoleAdmin {
		t.Errorf("expected admin role, got %s", updated.Role)
	}
}

func TestAdminUpdateUserRole_InvalidRole(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := st.GetUser("alice")

	body := `{"role": "superuser"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%d/role", alice.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d: %s", w.Code, w.Body.String())
	}
}
