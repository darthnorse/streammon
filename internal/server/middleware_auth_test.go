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

func TestRequireAuthManager_ValidAPIKey(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	plain, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	var gotUser *models.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if gotUser == nil {
		t.Fatal("expected synthetic user in context")
	}
	if gotUser.Role != models.RoleAdmin {
		t.Errorf("expected synthetic admin, got role=%s", gotUser.Role)
	}
	if !gotUser.APIKeyAuth {
		t.Error("expected APIKeyAuth=true on synthetic user")
	}
}

func TestRequireAuthManager_InvalidAPIKey_ReturnsAuthError_AndRateLimits(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run on bad key")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "sm_wrong")
	req.RemoteAddr = "10.0.0.1:1234"
	req = req.WithContext(context.WithValue(req.Context(), rawRemoteAddrKey{}, req.RemoteAddr))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	globalAuthRateLimiter.mu.Lock()
	count := len(globalAuthRateLimiter.attempts["10.0.0.1"])
	globalAuthRateLimiter.mu.Unlock()
	if count != 1 {
		t.Errorf("expected rate-limit counter incremented once, got %d", count)
	}
}

func TestRequireAuthManager_EmptyAPIKeyHeader_Rejects(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	user, _ := st.GetOrCreateUser("testuser")
	token, _ := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run on empty API key header")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (empty header must not fall through to cookie), got %d", w.Code)
	}
}

func TestRequireAuthManager_APIKeyRateLimitBlocksAfterThreshold(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Saturate the bucket via 10 bad attempts.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-API-Key", "sm_"+strings.Repeat("0", 64))
		req.RemoteAddr = "10.0.0.2:1234"
		req = req.WithContext(context.WithValue(req.Context(), rawRemoteAddrKey{}, req.RemoteAddr))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, w.Code)
		}
	}

	// 11th attempt — even with a *valid* key — must be 429.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", plain)
	req.RemoteAddr = "10.0.0.2:1234"
	req = req.WithContext(context.WithValue(req.Context(), rawRemoteAddrKey{}, req.RemoteAddr))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after rate-limit exceeded, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
}

func TestRequireAuthManager_APIKey_RejectsMalformedShape(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run on bad shape")
	}))

	for _, bad := range []string{"too-short", "abc_" + strings.Repeat("0", 64), "sm_short"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-API-Key", bad)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("input %q: expected 401, got %d", bad, w.Code)
		}
	}
}

func TestRequireAuthManager_APIKey_RejectsMultipleHeaders(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run when duplicates are sent")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("X-API-Key", plain)
	req.Header.Add("X-API-Key", plain)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on duplicate X-API-Key, got %d", w.Code)
	}
}

func TestRequireAuthManager_APIKey_RejectedBeforeSetup(t *testing.T) {
	resetAuthRateLimiter(t)
	st := newEmptyStore(t)
	mgr := auth.NewManager(st)

	// Even with a stored hash, no admin user exists (setup incomplete).
	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(plain, time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run when setup is required")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 before setup, got %d", w.Code)
	}
}

func TestRequireInteractiveSession_AllowsCookie(t *testing.T) {
	user := &models.User{ID: 1, Name: "u", Role: models.RoleAdmin}
	handler := RequireInteractiveSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil).WithContext(contextWithUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for cookie session, got %d", w.Code)
	}
}

func TestRequireInteractiveSession_RejectsAPIKey(t *testing.T) {
	user := &models.User{ID: -1, Name: "api", Role: models.RoleAdmin, APIKeyAuth: true}
	handler := RequireInteractiveSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for API-key principals")
	}))
	req := httptest.NewRequest("GET", "/", nil).WithContext(contextWithUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireAuthManager_BadAPIKey_DoesNotFallThroughToCookie(t *testing.T) {
	resetAuthRateLimiter(t)
	_, st := newTestServer(t)
	mgr := auth.NewManager(st)

	user, _ := st.GetOrCreateUser("testuser")
	token, _ := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	mw := RequireAuthManager(mgr)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run when API key is invalid")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "sm_wrong")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
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
