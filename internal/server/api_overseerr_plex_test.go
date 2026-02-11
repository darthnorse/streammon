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
)

// mockOverseerrWithPlexAuth creates a mock Overseerr server that handles both
// Plex auth (POST /api/v1/auth/plex) and request creation (POST /api/v1/request).
func mockOverseerrWithPlexAuth(t *testing.T, plexAuthID int, captured *map[string]any) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// auth/plex is called without API key (like a normal user login)
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			var body struct {
				AuthToken string `json:"authToken"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.AuthToken == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"id": plexAuthID, "email": "user@plex.tv"})
			return
		}
		// All other endpoints require API key
		if r.Header.Get("X-Api-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 0},
				"results":  []any{},
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

func TestOverseerrCreateRequest_PlexTokenAuth(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrWithPlexAuth(t, 77, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	// Enable Plex token storage and store a token for a viewer
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

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId in request body via Plex token auth")
	}
	if int(userID.(float64)) != 77 {
		t.Fatalf("expected userId=77, got %v", userID)
	}
}

func TestOverseerrCreateRequest_PlexTokenFallbackToEmail(t *testing.T) {
	// User has no Plex token but has email that matches Overseerr
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 42, "email": "viewer-fb@test.local"},
	}, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	// Plex tokens feature is enabled but no token stored for this user
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
	// Plex token feature disabled but user has email → should use email matching
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 55, "email": "viewer-dis@test.local"},
	}, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	// Token stored but feature disabled
	user, err := st.CreateLocalUser("viewer-dis", "viewer-dis@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	st.StoreProviderToken(user.ID, "plex", "ignored-token")
	// Note: SetStorePlexTokens(false) would delete tokens, so we store AFTER enabling
	st.SetStorePlexTokens(true)
	st.SetStorePlexTokens(false) // This deletes all tokens

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
	mock := mockOverseerrWithPlexAuth(t, 99, &receivedBody)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	// User has no plex token and no email → no attribution
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

func TestOverseerrCreateRequest_PlexTokenCached(t *testing.T) {
	plexAuthCalls := 0
	var receivedBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			plexAuthCalls++
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "email": "user@plex.tv"})
			return
		}
		if r.Header.Get("X-Api-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 0},
				"results":  []any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, ts.URL)
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("plex-cached", "", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	st.StoreProviderToken(user.ID, "plex", "test-plex-token")
	sessionToken, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// First request: should call Plex auth
	for i := 0; i < 3; i++ {
		receivedBody = nil
		body := `{"mediaType":"movie","mediaId":27205}`
		req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sessionToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i, w.Code)
		}
	}

	// Plex auth should only be called once (cached for the other two)
	if plexAuthCalls != 1 {
		t.Fatalf("expected 1 Plex auth call (cached), got %d", plexAuthCalls)
	}
}

func TestOverseerrCreateRequest_PlexTokenConcurrent(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/plex" {
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "email": "user@plex.tv"})
			return
		}
		if r.Header.Get("X-Api-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 0},
				"results":  []any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			capturedBodies = append(capturedBodies, body)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	// Verify no request was sent with userId: 0 (the concurrency bug)
	mu.Lock()
	defer mu.Unlock()
	for i, body := range capturedBodies {
		if uid, ok := body["userId"]; ok {
			if int(uid.(float64)) == 0 {
				t.Errorf("request %d: userId was 0 (concurrency bug)", i)
			}
		}
	}
}

func TestOverseerrCreateRequest_ExistingTests_StillPass(t *testing.T) {
	// Verify that existing email-based tests still work when no encryptor is present
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
