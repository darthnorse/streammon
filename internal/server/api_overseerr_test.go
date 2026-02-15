package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/store"
)

type mockOverseerrOpts struct {
	users           []map[string]any
	onGetRequests   func(w http.ResponseWriter, r *http.Request)
	onCreateRequest func(w http.ResponseWriter, r *http.Request)
}

func newMockOverseerr(t *testing.T, opts mockOverseerrOpts) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Invalid API key"}`))
			return
		}
		users := opts.users
		if users == nil {
			users = []map[string]any{}
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status":
			json.NewEncoder(w).Encode(map[string]string{"version": "1.33.2"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 1, "page": 1, "results": len(users)},
				"results":  users,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/search":
			json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "totalPages": 1, "totalResults": 1,
				"results": []map[string]any{{"id": 1, "mediaType": "movie", "title": "Test"}},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/discover/"):
			json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "totalPages": 1, "totalResults": 1,
				"results": []map[string]any{{"id": 2, "mediaType": "tv", "name": "Trending"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/movie/27205":
			json.NewEncoder(w).Encode(map[string]any{"id": 27205, "title": "Inception"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tv/1399":
			json.NewEncoder(w).Encode(map[string]any{"id": 1399, "name": "Breaking Bad"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tv/1399/season/1":
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "seasonNumber": 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request":
			if opts.onGetRequests != nil {
				opts.onGetRequests(w, r)
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"pageInfo": map[string]any{"page": 1, "pages": 1, "results": 1},
					"results":  []map[string]any{{"id": 1, "status": 2, "type": "movie"}},
				})
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(map[string]any{"total": 5, "pending": 2})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			if opts.onCreateRequest != nil {
				opts.onCreateRequest(w, r)
			} else {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				resp := map[string]any{"id": 10, "status": 2, "mediaType": body["mediaType"]}
				if uid, ok := body["userId"]; ok {
					resp["requestedBy"] = map[string]any{"id": uid}
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(resp)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request/1/approve":
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "status": 2})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request/1/decline":
			json.NewEncoder(w).Encode(map[string]any{"id": 1, "status": 3})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/request/1":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func mockOverseerr(t *testing.T) *httptest.Server {
	return newMockOverseerr(t, mockOverseerrOpts{})
}

func mockOverseerrWithUsers(t *testing.T, users []map[string]any) *httptest.Server {
	return newMockOverseerr(t, mockOverseerrOpts{users: users})
}

func mockOverseerrCaptureRequest(t *testing.T, users []map[string]any, captured *map[string]any) *httptest.Server {
	return newMockOverseerr(t, mockOverseerrOpts{
		users: users,
		onCreateRequest: func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(captured)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2})
		},
	})
}

func configureOverseerr(t *testing.T, st *store.Store, mockURL string) {
	t.Helper()
	if err := st.SetOverseerrConfig(store.OverseerrConfig{
		URL:     mockURL,
		APIKey:  "test-api-key",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func newOverseerrTestServer(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	mock := mockOverseerr(t)
	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)
	return srv, st
}

func TestOverseerrIntegrationSettings(t *testing.T) {
	testIntegrationSettingsCRUD(t, integrationTestConfig{
		name:           "overseerr",
		settingsPath:   "/api/settings/overseerr",
		testPath:       "/api/settings/overseerr/test",
		configuredPath: "/api/overseerr/configured",
		dataPath:       "/api/overseerr/search?query=test",
		configure:      configureOverseerr,
		getConfig:      func(st *store.Store) (store.IntegrationConfig, error) { return st.GetOverseerrConfig() },
		setConfig:      func(st *store.Store, c store.IntegrationConfig) error { return st.SetOverseerrConfig(c) },
		mockServer:     mockOverseerr,
	})
}

func TestOverseerrConfigured_ViewerCanAccess(t *testing.T) {
	srv, st := newTestServer(t)

	mock := mockOverseerr(t)
	configureOverseerr(t, st, mock.URL)

	viewerToken := createViewerSession(t, st, "viewer-cfg")

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/configured", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]bool
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp["configured"] {
		t.Fatal("expected configured=true when Overseerr is set up")
	}
}

func TestOverseerrSearch_NoConfig(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/search?query=test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrSearch_MissingQuery(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/search", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrSearch_Success(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/search?query=test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		TotalResults int `json:"totalResults"`
	}
	json.NewDecoder(w.Body).Decode(&result)
	if result.TotalResults != 1 {
		t.Fatalf("expected 1 result, got %d", result.TotalResults)
	}
}

func TestOverseerrDiscoverCategories(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	categories := []string{
		"trending",
		"movies",
		"movies/upcoming",
		"tv",
		"tv/upcoming",
	}

	for _, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/overseerr/discover/"+cat, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d: %s", cat, w.Code, w.Body.String())
			}
		})
	}
}

func TestOverseerrDiscoverInvalidCategory(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	invalid := []string{"evil", "../admin", "movies/evil", "tv/../secrets"}
	for _, cat := range invalid {
		t.Run(cat, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/overseerr/discover/"+cat, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for %q, got %d: %s", cat, w.Code, w.Body.String())
			}
		})
	}
}

func TestOverseerrGetMovie(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/movie/27205", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		Title string `json:"title"`
	}
	json.NewDecoder(w.Body).Decode(&result)
	if result.Title != "Inception" {
		t.Fatalf("expected Inception, got %s", result.Title)
	}
}

func TestOverseerrGetMovie_InvalidID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/movie/abc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrGetTV(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/tv/1399", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrGetTVSeason(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/tv/1399/season/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?take=10&filter=all&sort=added", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_InvalidFilter(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?filter=evil", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid filter, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_InvalidSort(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?sort=evil", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sort, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_TakeCapped(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?take=999999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrRequestCount(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests/count", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var counts struct {
		Total int `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&counts)
	if counts.Total != 5 {
		t.Fatalf("expected 5 total, got %d", counts.Total)
	}
}

func TestOverseerrCreateRequest_Movie(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_TV(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	body := `{"mediaType":"tv","mediaId":1399,"seasons":[1,2,3]}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_InvalidMediaType(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"mediaType":"person","mediaId":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_MissingMediaID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"mediaType":"movie"}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_InvalidJSON(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_ExtraFieldsStripped(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, nil, &receivedBody)

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	body := `{"mediaType":"movie","mediaId":27205,"userId":999,"rootFolder":"/evil","serverId":42}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if _, ok := receivedBody["userId"]; ok {
		t.Fatal("expected userId to be stripped from forwarded body")
	}
	if _, ok := receivedBody["rootFolder"]; ok {
		t.Fatal("expected rootFolder to be stripped from forwarded body")
	}
	if _, ok := receivedBody["serverId"]; ok {
		t.Fatal("expected serverId to be stripped from forwarded body")
	}
}

func TestOverseerrApproveRequest(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/approve", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeclineRequest(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/decline", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeleteRequest(t *testing.T) {
	srv, _ := newOverseerrTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/overseerr/requests/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_InjectsUserID(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 42, "email": "viewer@test.local"},
		{"id": 99, "email": "other@example.com"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "viewer-attr", "viewer@test.local")

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId in request body")
	}
	if int(userID.(float64)) != 42 {
		t.Fatalf("expected userId=42, got %v", userID)
	}
}

func TestOverseerrCreateRequest_NoMatchFallsBack(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 99, "email": "someone-else@example.com"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "viewer-nomatch", "nomatch@test.local")

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if _, ok := receivedBody["userId"]; ok {
		t.Fatal("expected no userId when user has no Overseerr match")
	}
}

func TestOverseerrCreateRequest_ClientUserIdStripped(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 99, "email": "someone-else@example.com"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "viewer-strip", "nomatch@test.local")

	body := `{"mediaType":"movie","mediaId":27205,"userId":999}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if uid, ok := receivedBody["userId"]; ok {
		t.Fatalf("expected client userId to be stripped, but got userId=%v", uid)
	}
}

func TestOverseerrCreateRequest_CaseInsensitiveEmail(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 7, "email": "Alice@Example.COM"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "alice-ci", "alice@example.com")

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId in request body for case-insensitive email match")
	}
	if int(userID.(float64)) != 7 {
		t.Fatalf("expected userId=7, got %v", userID)
	}
}

func TestOverseerrCreateRequest_AdminEmailResolved(t *testing.T) {
	var receivedBody map[string]any
	mock := mockOverseerrCaptureRequest(t, []map[string]any{
		{"id": 5, "email": "admin@test.local"},
	}, &receivedBody)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	userID, ok := receivedBody["userId"]
	if !ok {
		t.Fatal("expected userId in request body for admin user")
	}
	if int(userID.(float64)) != 5 {
		t.Fatalf("expected userId=5, got %v", userID)
	}
}

func TestOverseerrUserCache_InvalidatedOnSettingsChange(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       func(mockURL string) string
		wantStatus int
	}{
		{"update", http.MethodPut, func(u string) string { return `{"url":"` + u + `","api_key":"test-api-key"}` }, http.StatusOK},
		{"delete", http.MethodDelete, nil, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := mockOverseerrWithUsers(t, []map[string]any{
				{"id": 42, "email": "admin@test.local"},
			})
			srv, st := newTestServerWrapped(t)
			configureOverseerr(t, st, mock.URL)
			srv.Unwrap().resolveOverseerrUserID(context.Background(), "admin@test.local")

			var req *http.Request
			if tt.body != nil {
				req = httptest.NewRequest(tt.method, "/api/settings/overseerr", strings.NewReader(tt.body(mock.URL)))
			} else {
				req = httptest.NewRequest(tt.method, "/api/settings/overseerr", nil)
			}
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			srv.Unwrap().overseerrUsers.mu.RLock()
			expired := srv.Unwrap().overseerrUsers.expiresAt.IsZero()
			mapCleared := srv.Unwrap().overseerrUsers.emailToID == nil
			srv.Unwrap().overseerrUsers.mu.RUnlock()
			if !expired || !mapCleared {
				t.Fatal("expected cache to be invalidated")
			}
		})
	}
}

func TestOverseerrListRequests_ViewerFiltered(t *testing.T) {
	var capturedRequestedBy string
	mock := newMockOverseerr(t, mockOverseerrOpts{
		users: []map[string]any{{"id": 42, "email": "viewer-filter@test.local"}},
		onGetRequests: func(w http.ResponseWriter, r *http.Request) {
			capturedRequestedBy = r.URL.Query().Get("requestedBy")
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"page": 1, "pages": 1, "results": 1},
				"results":  []map[string]any{{"id": 1, "status": 2, "type": "movie"}},
			})
		},
	})

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSession(t, st, "viewer-filter")

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if capturedRequestedBy != "42" {
		t.Fatalf("expected requestedBy=42, got %q", capturedRequestedBy)
	}
}

func TestOverseerrListRequests_ViewerNoEmail(t *testing.T) {
	mock := mockOverseerr(t)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSessionWithEmail(t, st, "viewer-noemail", "")

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		PageInfo struct{ Results int } `json:"pageInfo"`
		Results  []json.RawMessage    `json:"results"`
	}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result.Results) != 0 {
		t.Fatalf("expected empty results for viewer with no email, got %d", len(result.Results))
	}
}

func TestOverseerrListRequests_ViewerNoOverseerrAccount(t *testing.T) {
	// Overseerr has no user with this email
	mock := mockOverseerrWithUsers(t, []map[string]any{
		{"id": 99, "email": "someone-else@example.com"},
	})

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSession(t, st, "viewer-nomatch")

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		PageInfo struct{ Results int } `json:"pageInfo"`
		Results  []json.RawMessage    `json:"results"`
	}
	json.NewDecoder(w.Body).Decode(&result)
	if len(result.Results) != 0 {
		t.Fatalf("expected empty results for unresolvable viewer, got %d", len(result.Results))
	}
}

func TestOverseerrListRequests_AdminUnfiltered(t *testing.T) {
	var capturedRequestedBy string
	mock := newMockOverseerr(t, mockOverseerrOpts{
		onGetRequests: func(w http.ResponseWriter, r *http.Request) {
			capturedRequestedBy = r.URL.Query().Get("requestedBy")
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"page": 1, "pages": 1, "results": 1},
				"results":  []map[string]any{{"id": 1, "status": 2, "type": "movie"}},
			})
		},
	})

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests", nil)
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if capturedRequestedBy != "" {
		t.Fatalf("expected no requestedBy for admin, got %q", capturedRequestedBy)
	}
}

func TestOverseerrRequestCount_ViewerForbidden(t *testing.T) {
	mock := mockOverseerr(t)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSession(t, st, "viewer-count")

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests/count", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer on count endpoint, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrAdminActions_ViewerForbidden(t *testing.T) {
	mock := mockOverseerr(t)

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)
	viewerToken := createViewerSession(t, st, "viewer")

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"approve", http.MethodPost, "/api/overseerr/requests/1/approve"},
		{"decline", http.MethodPost, "/api/overseerr/requests/1/decline"},
		{"delete", http.MethodDelete, "/api/overseerr/requests/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for viewer on %s, got %d: %s", tt.name, w.Code, w.Body.String())
			}
		})
	}
}
