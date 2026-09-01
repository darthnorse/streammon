package server

import (
	"context"
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

// Regression: a user whose stored Plex token Overseerr rejected used to
// dead-end on "upstream service error". Attribution no longer touches Plex
// tokens, so the stored token is irrelevant. The shared mock 404s any path it
// does not know, /auth/plex included, so a return to Plex sign-in fails here.
func TestOverseerrCreateRequest_StalePlexTokenIsIrrelevant(t *testing.T) {
	var received createCapture
	mock := newMockOverseerr(t, mockOverseerrOpts{
		users: []map[string]any{{"id": 42, "email": "stale@test.local"}},
		onCreateRequest: func(w http.ResponseWriter, r *http.Request) {
			received.APIUser = r.Header.Get("X-API-User")
			json.NewDecoder(r.Body).Decode(&received.Body)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 1})
		},
	})

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)
	st.SetStorePlexTokens(true)

	user, err := st.CreateLocalUser("stale-token", "stale@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.StoreProviderToken(user.ID, "plex", "revoked-token"); err != nil {
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
	if received.APIUser != "42" {
		t.Fatalf("expected X-API-User=42, got %q", received.APIUser)
	}
	if _, ok := received.Body["userId"]; ok {
		t.Fatal("body must not carry userId; Overseerr evaluates approval against the API key owner when it does")
	}
}

// An admin with no Overseerr counterpart still gets through on the bare API
// key — only non-admins are required to be resolvable.
func TestOverseerrCreateRequest_AdminWithoutOverseerrAccount(t *testing.T) {
	var received createCapture
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 99, "email": "someone-else@test.local"},
	}, &received)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, mock.URL)

	user, err := st.CreateLocalUser("admin-unres", "admin-unres@test.local", "", models.RoleAdmin)
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
	if received.APIUser != "" {
		t.Fatalf("expected no impersonation header, got %q", received.APIUser)
	}
}

// Overseerr evaluates the impersonated user's own permissions and quota, so a
// 4xx is about their account — reporting it as 502 sends them chasing a broken
// integration instead of their quota.
func TestOverseerrCreateRequest_UpstreamStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		upstream   int
		wantStatus int
		wantBody   string
	}{
		{"quota or permission denied", http.StatusForbidden, http.StatusForbidden, "quota"},
		{"already requested", http.StatusConflict, http.StatusConflict, "already been requested"},
		{"other rejection", http.StatusBadRequest, http.StatusUnprocessableEntity, "rejected"},
		{"overseerr broken", http.StatusInternalServerError, http.StatusBadGateway, "upstream service error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockOverseerr(t, mockOverseerrOpts{
				users: []map[string]any{{"id": 42, "email": "quota@test.local"}},
				onCreateRequest: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.upstream)
					w.Write([]byte(`{"message":"Movie Quota exceeded."}`))
				},
			})

			srv, st := newTestServer(t)
			configureOverseerr(t, st, mock.URL)
			viewerToken := createViewerSessionWithEmail(t, st, "viewer-quota", "quota@test.local")

			body := `{"mediaType":"movie","mediaId":27205}`
			req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("upstream %d: expected %d, got %d: %s", tt.upstream, tt.wantStatus, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Fatalf("expected body to mention %q, got %s", tt.wantBody, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "Movie Quota exceeded.") {
				t.Fatal("upstream response body must not be echoed to the client")
			}
		})
	}
}

// A failed user-list fetch must not read as "your account is not linked" —
// that sends users to fix an email that was never the problem.
func TestOverseerrCreateRequest_ResolveFailureIsNotAnIdentityError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, ts.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "viewer-upstream", "upstream@test.local")

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when the user list cannot be fetched, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "no matching Overseerr account") {
		t.Fatal("an outage must not be reported as an unlinked account")
	}
}

// Regression: the shared refresh must outlive the request that triggered it.
// A client disconnecting mid-fetch previously armed a 30s global backoff that
// made every other user look unlinked while Overseerr was healthy.
func TestOverseerrResolve_CallerCancellationDoesNotPoisonCache(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/user" {
			once.Do(func() { <-release }) // hold the first fetch until the caller is gone
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 1},
				"results":  []map[string]any{{"id": 42, "email": "cancel@test.local"}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, ts.URL)

	cancelCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.resolveOverseerrUserID(cancelCtx, "cancel@test.local")
	}()

	// Let the fetch start, then abandon it the way a disconnecting client would.
	time.Sleep(50 * time.Millisecond)
	cancel()
	close(release)
	<-done

	id, ok, err := srv.resolveOverseerrUserID(context.Background(), "cancel@test.local")
	if err != nil {
		t.Fatalf("a caller's cancellation must not disable resolution for everyone: %v", err)
	}
	if !ok || id != 42 {
		t.Fatalf("expected id=42 resolved, got id=%d ok=%v", id, ok)
	}
}

func TestOverseerrCreateRequest_ConcurrentAttribution(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 1},
				"results":  []map[string]any{{"id": 42, "email": "conc@test.local"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			mu.Lock()
			seen[r.Header.Get("X-API-User")]++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)

	srv, st := newTestServerWithEncryptor(t)
	configureOverseerr(t, st, ts.URL)
	sessionToken := createViewerSessionWithEmail(t, st, "viewer-conc", "conc@test.local")

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
	if seen["42"] != 10 {
		t.Fatalf("expected all 10 requests attributed to user 42, got %v", seen)
	}
}
