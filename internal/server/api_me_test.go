package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

type meTestResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	HasPassword bool   `json:"has_password"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func TestHandleMe_ReturnsUser(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("testpass1")
	user, err := st.CreateLocalUser("alice", "alice@example.com", hash, models.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/me", nil)
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got meTestResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.Name != "alice" {
		t.Errorf("got name %q, want alice", got.Name)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("got email %q, want alice@example.com", got.Email)
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

func TestHandleMe_HasPassword_True(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("testpass123")
	user, err := st.CreateLocalUser("passuser", "pass@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/me", nil)
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got meTestResponse
	json.NewDecoder(w.Body).Decode(&got)
	if !got.HasPassword {
		t.Error("expected has_password=true")
	}
}

func TestHandleMe_HasPassword_False(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	user, err := st.CreateLocalUser("oauthuser", "oauth@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/me", nil)
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got meTestResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.HasPassword {
		t.Error("expected has_password=false")
	}
}

func TestHandleUpdateProfile_UpdatesEmail(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	user, err := st.CreateLocalUser("emailuser", "old@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"email":"new@test.local"}`
	req := httptest.NewRequest("PUT", "/api/me", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleUpdateProfile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got meTestResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.Email != "new@test.local" {
		t.Errorf("got email %q, want new@test.local", got.Email)
	}

	// Verify in DB
	updated, err := st.GetUser("emailuser")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email != "new@test.local" {
		t.Errorf("DB email %q, want new@test.local", updated.Email)
	}
}

func TestHandleUpdateProfile_InvalidEmail(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	user, err := st.CreateLocalUser("badmail", "old@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest("PUT", "/api/me", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleUpdateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateProfile_EmailConflict(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	// Create two users
	user1, err := st.CreateLocalUser("user1", "user1@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateLocalUser("user2", "user2@test.local", "", models.RoleViewer); err != nil {
		t.Fatal(err)
	}

	// user1 tries to take user2's email
	body := `{"email":"user2@test.local"}`
	req := httptest.NewRequest("PUT", "/api/me", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user1)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleUpdateProfile(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var got errorResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.Error != "email already in use" {
		t.Errorf("got error %q, want 'email already in use'", got.Error)
	}
}

func TestHandleUpdateProfile_EmptyBody(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	user, err := st.CreateLocalUser("emptybody", "eb@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("PUT", "/api/me", strings.NewReader(""))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleUpdateProfile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestHandleChangePassword_Success(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("oldpass123")
	user, err := st.CreateLocalUser("pwduser", "pwd@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"current_password":"oldpass123","new_password":"newpass456"}`
	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify new password works
	newHash, _ := st.GetPasswordHashByUserID(user.ID)
	valid, _ := auth.VerifyPassword("newpass456", newHash)
	if !valid {
		t.Error("new password should verify")
	}
}

func TestHandleChangePassword_WrongCurrentPassword(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("correct123")
	user, err := st.CreateLocalUser("wrongpwd", "wrong@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"current_password":"wrongpass1","new_password":"newpass456"}`
	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleChangePassword_NoPasswordSet(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	user, err := st.CreateLocalUser("nopwd", "nopwd@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"current_password":"anything1","new_password":"newpass456"}`
	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleChangePassword_TooShortNewPassword(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("oldpass123")
	user, err := st.CreateLocalUser("shortpwd", "short@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"current_password":"oldpass123","new_password":"short"}`
	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(body))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var got errorResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.Error != "password must be at least 8 characters" {
		t.Errorf("got error %q, want password validation message", got.Error)
	}
}

func TestHandleChangePassword_RevokesOtherSessions(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("oldpass123")
	user, err := st.CreateLocalUser("revokeuser", "revoke@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	expires := time.Now().UTC().Add(24 * time.Hour)
	currentToken, err := st.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("creating current session: %v", err)
	}
	otherToken1, err := st.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("creating other session 1: %v", err)
	}
	otherToken2, err := st.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("creating other session 2: %v", err)
	}

	body := `{"current_password":"oldpass123","new_password":"newpass456"}`
	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: currentToken})
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Current session should survive
	if _, err := st.GetSessionUser(currentToken); err != nil {
		t.Errorf("current session should still be valid: %v", err)
	}

	// Other sessions should be revoked
	if _, err := st.GetSessionUser(otherToken1); err == nil {
		t.Error("other session 1 should have been revoked")
	}
	if _, err := st.GetSessionUser(otherToken2); err == nil {
		t.Error("other session 2 should have been revoked")
	}
}

func TestHandleChangePassword_EmptyBody(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	hash, _ := auth.HashPassword("oldpass123")
	user, err := st.CreateLocalUser("emptypwd", "emptypwd@test.local", hash, models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/me/password", strings.NewReader(""))
	ctx := contextWithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	srv.handleChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", w.Code)
	}
}
