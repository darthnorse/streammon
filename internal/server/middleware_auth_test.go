package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func contextWithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func TestRequireAuthManager_NoCookie_Returns401(t *testing.T) {
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuthManager_ValidSession(t *testing.T) {
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	user, _ := st.GetOrCreateUser("testuser")
	token, _ := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	mw := RequireAuthManager(mgr)
	var gotUser *models.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("expected user in context")
	}
	if gotUser.Name != "testuser" {
		t.Errorf("expected testuser, got %s", gotUser.Name)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	mw := RequireRole(models.RoleAdmin)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	mw := RequireRole(models.RoleAdmin)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	ctx := contextWithUser(req.Context(), &models.User{Name: "admin", Role: models.RoleAdmin})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireSetup_AllowsWhenNoUsers(t *testing.T) {
	st := newEmptyStore(t)
	mgr := auth.NewManager(st)

	mw := RequireSetup(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no users exist, got %d", w.Code)
	}
}

func TestRequireSetup_BlocksWhenUsersExist(t *testing.T) {
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	st.GetOrCreateUser("existing_user")

	mw := RequireSetup(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 when users exist, got %d", w.Code)
	}
}

func TestRequireSetupComplete_BlocksWhenNoUsers(t *testing.T) {
	st := newEmptyStore(t)
	mgr := auth.NewManager(st)

	mw := RequireSetupComplete(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 when no users exist, got %d", w.Code)
	}
}

func TestRequireSetupComplete_AllowsWhenUsersExist(t *testing.T) {
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	st.GetOrCreateUser("existing_user")

	mw := RequireSetupComplete(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when users exist, got %d", w.Code)
	}
}

func TestBodyLimitEnforced_AuthRoutes(t *testing.T) {
	srv, _ := newTestServer(t)

	// 2MB body exceeds the 1MB limit
	oversized := strings.Repeat("x", 2<<20)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/auth/local/login"},
		{http.MethodPost, "/api/setup/local"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(oversized))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			// Read the body to trigger MaxBytesReader error
			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)

			// Should get an error status (400 or 413), not 200
			if resp.StatusCode == http.StatusOK {
				t.Errorf("expected error for oversized body on %s, got 200: %s", rt.path, body)
			}
		})
	}
}
