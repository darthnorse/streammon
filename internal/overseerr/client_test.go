package overseerr

import (
	"context"
	"encoding/json"
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
