package radarr

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
		{"http://localhost:7878", false},
		{"https://radarr.example.com", false},
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
	c, err := NewClient("http://localhost:7878", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL != "http://localhost:7878" {
		t.Fatalf("expected baseURL http://localhost:7878, got %s", c.BaseURL)
	}
}

func TestNewClientTrimsSlash(t *testing.T) {
	c, err := NewClient("http://localhost:7878/", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL != "http://localhost:7878" {
		t.Fatalf("expected trailing slash trimmed, got %s", c.BaseURL)
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
		if r.URL.Path != "/api/v3/system/status" {
			t.Errorf("expected path /api/v3/system/status, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header test-key, got %s", r.Header.Get("X-Api-Key"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":"5.0.0"}`))
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

func TestLookupMovieByTMDB(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("expected path /api/v3/movie, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("tmdbId") != "27205" {
			t.Errorf("expected tmdbId=27205, got %s", r.URL.Query().Get("tmdbId"))
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 42, "title": "Inception", "tmdbId": 27205},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.LookupMovieByTMDB(context.Background(), "27205")
	if err != nil {
		t.Fatalf("LookupMovieByTMDB: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected movie ID 42, got %d", id)
	}
}

func TestLookupMovieByTMDBNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.LookupMovieByTMDB(context.Background(), "99999")
	if err != nil {
		t.Fatalf("LookupMovieByTMDB: %v", err)
	}
	if id != 0 {
		t.Fatalf("expected 0 for not found, got %d", id)
	}
}

func TestDeleteMovie(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie/42" {
			t.Errorf("expected path /api/v3/movie/42, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("deleteFiles") != "true" {
			t.Errorf("expected deleteFiles=true, got %s", r.URL.Query().Get("deleteFiles"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.DeleteMovie(context.Background(), 42, true); err != nil {
		t.Fatalf("DeleteMovie: %v", err)
	}
}

func TestDeleteMovieFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Movie not found"}`))
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.DeleteMovie(context.Background(), 999, true); err == nil {
		t.Fatal("expected error for 404 response")
	}
}
