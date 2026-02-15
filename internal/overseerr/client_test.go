package overseerr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://localhost:5055", false},
		{"https://overseerr.example.com", false},
		{"", true},
		{"ftp://bad.com", true},
		{"not-a-url", true},
	}
	for _, tt := range tests {
		err := ValidateURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURL(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestNewClient(t *testing.T) {
	c, err := NewClient("http://localhost:5055", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.baseURL != "http://localhost:5055" {
		t.Fatalf("expected baseURL http://localhost:5055, got %s", c.baseURL)
	}
}

func TestNewClientTrimsSlash(t *testing.T) {
	c, err := NewClient("http://localhost:5055/", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.baseURL != "http://localhost:5055" {
		t.Fatalf("expected trailing slash trimmed, got %s", c.baseURL)
	}
}

func TestNewClientInvalidURL(t *testing.T) {
	_, err := NewClient("", "test-key")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestTestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status" {
			t.Errorf("expected path /api/v1/status, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header test-key, got %s", r.Header.Get("X-Api-Key"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":"1.33.2"}`))
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Invalid API key"}`))
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "bad-key")
	if err := c.TestConnection(context.Background()); err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("expected path /api/v1/search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "Inception" {
			t.Errorf("expected query=Inception, got %s", r.URL.Query().Get("query"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"page":         1,
			"totalPages":   1,
			"totalResults": 1,
			"results":      []map[string]any{{"id": 27205, "mediaType": "movie", "title": "Inception"}},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.Search(context.Background(), "Inception", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	var result struct {
		TotalResults int `json:"totalResults"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.TotalResults != 1 {
		t.Fatalf("expected 1 result, got %d", result.TotalResults)
	}
}

func TestGetMovie(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/movie/27205" {
			t.Errorf("expected path /api/v1/movie/27205, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 27205, "title": "Inception"})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.GetMovie(context.Background(), 27205)
	if err != nil {
		t.Fatalf("GetMovie: %v", err)
	}

	var movie struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &movie); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if movie.Title != "Inception" {
		t.Fatalf("expected Inception, got %s", movie.Title)
	}
}

func TestGetTV(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tv/1399" {
			t.Errorf("expected path /api/v1/tv/1399, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 1399, "name": "Breaking Bad"})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.GetTV(context.Background(), 1399)
	if err != nil {
		t.Fatalf("GetTV: %v", err)
	}

	var show struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &show); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if show.Name != "Breaking Bad" {
		t.Fatalf("expected Breaking Bad, got %s", show.Name)
	}
}

func TestCreateRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("expected path /api/v1/request, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "status": 2})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	reqBody, _ := json.Marshal(map[string]any{"mediaType": "movie", "mediaId": 27205})
	data, err := c.CreateRequest(context.Background(), reqBody)
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ID != 1 {
		t.Fatalf("expected request ID 1, got %d", result.ID)
	}
}

func TestUpdateRequestStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/request/5/approve" {
			t.Errorf("expected path /api/v1/request/5/approve, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 5, "status": 2})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	_, err := c.UpdateRequestStatus(context.Background(), 5, "approve")
	if err != nil {
		t.Fatalf("UpdateRequestStatus: %v", err)
	}
}

func TestUpdateRequestStatusInvalid(t *testing.T) {
	c, _ := NewClient("http://localhost:5055", "test-key")
	_, err := c.UpdateRequestStatus(context.Background(), 5, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestDeleteRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.DeleteRequest(context.Background(), 1); err != nil {
		t.Fatalf("DeleteRequest: %v", err)
	}
}

func TestListUsers_SinglePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			t.Errorf("expected path /api/v1/user, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"pageInfo": map[string]any{"pages": 1, "page": 1, "results": 2},
			"results": []map[string]any{
				{"id": 1, "email": "alice@example.com"},
				{"id": 2, "email": "bob@example.com"},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	users, err := c.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Email != "alice@example.com" || users[0].ID != 1 {
		t.Fatalf("unexpected user[0]: %+v", users[0])
	}
	if users[1].Email != "bob@example.com" || users[1].ID != 2 {
		t.Fatalf("unexpected user[1]: %+v", users[1])
	}
}

func TestListUsers_Paginated(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		skip := r.URL.Query().Get("skip")
		switch skip {
		case "", "0":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 2, "page": 1, "results": 50},
				"results": func() []map[string]any {
					users := make([]map[string]any, 50)
					for i := range users {
						users[i] = map[string]any{"id": i + 1, "email": fmt.Sprintf("user%d@example.com", i+1)}
					}
					return users
				}(),
			})
		case "50":
			json.NewEncoder(w).Encode(map[string]any{
				"pageInfo": map[string]any{"pages": 2, "page": 2, "results": 10},
				"results": func() []map[string]any {
					users := make([]map[string]any, 10)
					for i := range users {
						users[i] = map[string]any{"id": 51 + i, "email": fmt.Sprintf("user%d@example.com", 51+i)}
					}
					return users
				}(),
			})
		default:
			t.Errorf("unexpected skip value: %s", skip)
		}
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	users, err := c.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 60 {
		t.Fatalf("expected 60 users, got %d", len(users))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestListUsers_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"server error"}`))
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	_, err := c.ListUsers(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFindRequestByTMDB_Found(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/movie/27205" {
			t.Errorf("expected path /api/v1/movie/27205, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": 27205,
			"mediaInfo": map[string]any{
				"requests": []map[string]any{
					{"id": 10},
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.FindRequestByTMDB(context.Background(), 27205, "movie")
	if err != nil {
		t.Fatalf("FindRequestByTMDB: %v", err)
	}
	if id != 10 {
		t.Fatalf("expected request ID 10, got %d", id)
	}
}

func TestFindRequestByTMDB_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id": 99999,
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.FindRequestByTMDB(context.Background(), 99999, "movie")
	if err != nil {
		t.Fatalf("FindRequestByTMDB: %v", err)
	}
	if id != 0 {
		t.Fatalf("expected 0 for not found, got %d", id)
	}
}

func TestFindRequestByTMDB_UsesCorrectPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tv/12345" {
			t.Errorf("expected path /api/v1/tv/12345, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": 12345,
			"mediaInfo": map[string]any{
				"requests": []map[string]any{
					{"id": 20},
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.FindRequestByTMDB(context.Background(), 12345, "tv")
	if err != nil {
		t.Fatalf("FindRequestByTMDB: %v", err)
	}
	if id != 20 {
		t.Fatalf("expected request ID 20, got %d", id)
	}
}

func TestRequestCount(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"total": 10, "pending": 3, "approved": 5, "available": 2,
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.RequestCount(context.Background())
	if err != nil {
		t.Fatalf("RequestCount: %v", err)
	}

	var counts struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(data, &counts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if counts.Total != 10 {
		t.Fatalf("expected 10 total, got %d", counts.Total)
	}
}
