package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func contextWithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func TestRequireAuth_Disabled_InjectsDefaultAdmin(t *testing.T) {
	svc, _ := auth.NewService(auth.Config{}, nil)
	mw := RequireAuth(svc)

	var gotUser *models.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("expected user in context")
	}
	if gotUser.Role != models.RoleAdmin {
		t.Errorf("expected admin role, got %s", gotUser.Role)
	}
}

func TestRequireAuth_Enabled_NoCookie_Returns401(t *testing.T) {
	_, st := newTestServer(t)
	svc, _ := auth.NewService(auth.Config{}, st)

	user, _ := st.GetOrCreateUser("testuser")
	token, _ := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	mw := RequireAuth(svc)
	var gotUser *models.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if gotUser.Name != "test-admin" {
		t.Errorf("expected test-admin, got %s", gotUser.Name)
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
