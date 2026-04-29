package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func resetAuthRateLimiter(t *testing.T) {
	t.Helper()
	globalAuthRateLimiter.mu.Lock()
	defer globalAuthRateLimiter.mu.Unlock()
	globalAuthRateLimiter.attempts = make(map[string][]time.Time)
}

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

// Reproduces the X-Forwarded-For bypass: middleware.RealIP rewrites RemoteAddr
// from the header, so a client rotating X-Forwarded-For per request used to
// land in fresh limiter buckets and never hit the cap. With the raw-socket
// capture in place, all attempts key on the actual peer.
func TestRateLimitAuth_IgnoresSpoofedXForwardedFor(t *testing.T) {
	resetAuthRateLimiter(t)

	r := chi.NewRouter()
	r.Use(CaptureRawRemoteAddr)
	r.Use(middleware.RealIP)
	r.With(RateLimitAuth).Post("/login", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"bad creds"}`, http.StatusUnauthorized)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	var lastStatus int
	for i := 0; i < 11; i++ {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/login", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		lastStatus = resp.StatusCode
	}

	if lastStatus != http.StatusTooManyRequests {
		t.Errorf("11th attempt with rotating X-Forwarded-For: status = %d, want 429", lastStatus)
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
