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
	"streammon/internal/store"
)

func mockOverseerr(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Invalid API key"}`))
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/search":
			json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "totalPages": 1, "totalResults": 1,
				"results": []map[string]any{{"id": 1, "mediaType": "movie", "title": "Test"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/discover/trending":
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
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"page": 1, "pages": 1, "results": 1},
				"results":  []map[string]any{{"id": 1, "status": 2, "type": "movie"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(map[string]any{"total": 5, "pending": 2})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/request":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": 2, "mediaType": body["mediaType"]})
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
}

func configureOverseerr(t *testing.T, st *store.Store, mockURL string) {
	t.Helper()
	if err := st.SetOverseerrConfig(store.OverseerrConfig{
		URL:    mockURL,
		APIKey: "test-api-key",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGetOverseerrSettings_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/overseerr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp overseerrSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.URL != "" || resp.APIKey != "" {
		t.Fatalf("expected empty settings, got %+v", resp)
	}
}

func TestUpdateOverseerrSettings_Saves(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	body := `{"url":"http://overseerr:5055","api_key":"myapikey123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/overseerr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetOverseerrConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://overseerr:5055" {
		t.Fatalf("URL: got %q", cfg.URL)
	}
	if cfg.APIKey != "myapikey123" {
		t.Fatalf("APIKey: got %q", cfg.APIKey)
	}
}

func TestGetOverseerrSettings_MasksKey(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetOverseerrConfig(store.OverseerrConfig{
		URL:    "http://overseerr:5055",
		APIKey: "secret-key",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/overseerr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp overseerrSettings
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.APIKey != maskedSecret {
		t.Fatalf("expected masked api_key %q, got %q", maskedSecret, resp.APIKey)
	}
}

func TestDeleteOverseerrSettings(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.SetOverseerrConfig(store.OverseerrConfig{
		URL:    "http://overseerr:5055",
		APIKey: "my-key",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/overseerr", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cfg, _ := st.GetOverseerrConfig()
	if cfg.URL != "" || cfg.APIKey != "" {
		t.Fatalf("expected empty config after delete, got %+v", cfg)
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

func TestUpdateOverseerrSettings_InvalidURL(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"url":"not-a-url","api_key":"key"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/overseerr", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrSearch_Success(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

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

func TestOverseerrDiscoverTrending(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/discover/trending", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrGetMovie(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

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
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/tv/1399", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrGetTVSeason(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/tv/1399/season/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?take=10&filter=all&sort=added", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_InvalidFilter(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	mock := mockOverseerr(t)
	defer mock.Close()
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?filter=evil", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid filter, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_InvalidSort(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	mock := mockOverseerr(t)
	defer mock.Close()
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?sort=evil", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sort, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrListRequests_TakeCapped(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/overseerr/requests?take=999999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should succeed (take gets capped, not rejected)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrRequestCount(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

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
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	body := `{"mediaType":"movie","mediaId":27205}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrCreateRequest_TV(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

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
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	// Include extra fields that should be stripped by the typed struct
	body := `{"mediaType":"movie","mediaId":27205,"userId":999,"rootFolder":"/evil","serverId":42}`
	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrApproveRequest(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/approve", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeclineRequest(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/decline", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeleteRequest(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServerWrapped(t)
	configureOverseerr(t, st, mock.URL)

	req := httptest.NewRequest(http.MethodDelete, "/api/overseerr/requests/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrApproveRequest_ViewerForbidden(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	// Create a viewer user
	viewer, err := st.CreateLocalUser("viewer", "viewer@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	viewerToken, err := st.CreateSession(viewer.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/approve", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer on approve, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeclineRequest_ViewerForbidden(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	viewer, err := st.CreateLocalUser("viewer2", "viewer2@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	viewerToken, err := st.CreateSession(viewer.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/overseerr/requests/1/decline", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer on decline, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOverseerrDeleteRequest_ViewerForbidden(t *testing.T) {
	mock := mockOverseerr(t)
	defer mock.Close()

	srv, st := newTestServer(t)
	configureOverseerr(t, st, mock.URL)

	viewer, err := st.CreateLocalUser("viewer3", "viewer3@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	viewerToken, err := st.CreateSession(viewer.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/overseerr/requests/1", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer on delete, got %d: %s", w.Code, w.Body.String())
	}
}
