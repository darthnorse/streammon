package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
	"streammon/internal/store"
)

// mockOverseerrWithPlexSession creates a mock Overseerr that:
//   - POST /auth/plex: validates authToken, sets a session cookie, returns user ID
//   - POST /request with session cookie: accepts the request (user-level auth)
//   - POST /request with API key: accepts the request (admin-level auth)
//   - GET /user with API key: returns user list for email matching
func mockOverseerrWithPlexSession(t *testing.T, plexAuthID int, users []map[string]any, captured *map[string]any) *httptest.Server {
	t.Helper()
	const sessionValue = "mock-session-token"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			var body struct {
				AuthToken string `json:"authToken"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.AuthToken == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:  "connect.sid",
				Value: sessionValue,
				Path:  "/",
			})
			json.NewEncoder(w).Encode(map[string]any{"id": plexAuthID, "email": "user@plex.tv"})
			return
		}

		hasAPIKey := r.Header.Get("X-Api-Key") == "test-api-key"
		hasSession := false
		if c, err := r.Cookie("connect.sid"); err == nil && c.Value == sessionValue {
			hasSession = true
		}

		if !hasAPIKey && !hasSession {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": len(users)},
				"results":  users,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			if captured != nil {
				json.NewDecoder(r.Body).Decode(captured)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestIsOverseerrURLSafeForTokens(t *testing.T) {
	tests := []struct {
		url  string
		safe bool
	}{
		{"https://overseerr.example.com", true},
		{"https://127.0.0.1", true},
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:5055", true},
		{"http://localhost", true},
		{"http://localhost:5055", true},
		{"http://[::1]", true},
		{"http://[::1]:5055", true},
		{"http://192.168.1.50:5055", true},
		{"http://10.0.0.5:5055", true},
		{"http://172.16.0.10:5055", true},
		{"http://overseerr:5055", true},
		{"http://overseerr.example.com", false},
		{"http://127.0.0.1.evil.com", false},
		{"http://localhost.evil.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			srv, st := newTestServer(t)
			if tt.url != "" {
				st.SetOverseerrConfig(store.OverseerrConfig{
					URL: tt.url, APIKey: "k", Enabled: true,
				})
			}
			got := srv.isOverseerrURLSafeForTokens()
			if got != tt.safe {
				t.Errorf("isOverseerrURLSafeForTokens(%q) = %v, want %v", tt.url, got, tt.safe)
			}
		})
	}
}

func TestOverseerrCreateRequest_PlexTokenAuth(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrWithPlexSession(t, 77, nil, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("plex-user", "", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.StoreProviderToken(user.ID, "plex", "test-plex-token"); err != nil {
		t.Fatal(err)
	}
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// With session-based auth, no userId should be in the body
	if _, ok := receivedBody["userId"]; ok {
		t.Fatal("expected no userId in body when using Plex session auth")
	}
}

func TestOverseerrCreateRequest_PlexAuthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Invalid Plex token"}`))
			return
		}
		if r.Header.Get("X-Api-Key") == "test-api-key" {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/status" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, ts.URL)
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("plex-bad", "", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	st.StoreProviderToken(user.ID, "plex", "expired-token")
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_PlexTokenFallbackToEmail(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 42, "email": "viewer-fb@test.local"},
	}, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	// Plex tokens enabled but no token stored for this user → email fallback
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("viewer-fb", "viewer-fb@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId from email fallback")
	}
	if int(userID.(float64)) != 42 {
		t.Fatalf("expected userId=42 from email match, got %v", userID)
	}
}

func TestOverseerrCreateRequest_PlexTokenDisabledFallsBack(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 55, "email": "viewer-dis@test.local"},
	}, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	user, err := st.CreateLocalUser("viewer-dis", "viewer-dis@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	st.StoreProviderToken(user.ID, "plex", "ignored-token")
	st.SetStorePlexTokens(true)
	st.SetStorePlexTokens(false) // deletes all tokens

	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId from email fallback when plex tokens disabled")
	}
	if int(userID.(float64)) != 55 {
		t.Fatalf("expected userId=55, got %v", userID)
	}
}

func TestOverseerrCreateRequest_NoTokenNoEmail(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrWithPlexSession(t, 99, nil, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	user, err := st.CreateLocalUser("viewer-none", "", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if _, ok := receivedBody["userId"]; ok {
		t.Fatal("expected no userId when user has no token and no email")
	}
}

func TestOverseerrCreateRequest_PlexTokenConcurrent(t *testing.T) {
	var mu sync.Mutex
	successCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			http.SetCookie(w, &http.Cookie{Name: "connect.sid", Value: "sess", Path: "/"})
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "email": "user@plex.tv"})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/request" {
			mu.Lock()
			successCount++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, ts.URL)
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("plex-conc", "", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	st.StoreProviderToken(user.ID, "plex", "test-plex-token")
	sessionToken, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := `{"mediaType":"movie","mediaId":27205}`
			req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sessionToken})
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusCreated {
				t.Errorf("expected 201, got %d", w.Code)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if successCount != 10 {
		t.Fatalf("expected 10 successful requests, got %d", successCount)
	}
}

func TestOverseerrCreateRequest_ExistingTests_StillPass(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 42, "email": "viewer@test.local"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	user, err := st.CreateLocalUser("viewer-compat", "viewer@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId in request body for email match")
	}
	if int(userID.(float64)) != 42 {
		t.Fatalf("expected userId=42, got %v", userID)
	}
}
