package sonarr

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
		{"http://localhost:8989", false},
		{"https://sonarr.example.com", false},
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
	c, err := NewClient("http://localhost:8989", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL != "http://localhost:8989" {
		t.Fatalf("expected baseURL http://localhost:8989, got %s", c.BaseURL)
	}
}

func TestNewClientTrimsSlash(t *testing.T) {
	c, err := NewClient("http://localhost:8989/", "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL != "http://localhost:8989" {
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
		w.Write([]byte(`{"version":"4.0.0"}`))
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

func TestGetSeries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series/10" {
			t.Errorf("expected path /api/v3/series/10, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header test-key, got %s", r.Header.Get("X-Api-Key"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    10,
			"title": "Test Series",
			"year":  2024,
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.GetSeries(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}

	var series struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &series); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if series.Title != "Test Series" {
		t.Fatalf("expected Test Series, got %s", series.Title)
	}
}

func TestLookupSeriesByTVDB(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series" {
			t.Errorf("expected path /api/v3/series, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("tvdbId") != "12345" {
			t.Errorf("expected tvdbId=12345, got %s", r.URL.Query().Get("tvdbId"))
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 77, "title": "Test Show", "tvdbId": 12345},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.LookupSeriesByTVDB(context.Background(), "12345")
	if err != nil {
		t.Fatalf("LookupSeriesByTVDB: %v", err)
	}
	if id != 77 {
		t.Fatalf("expected series ID 77, got %d", id)
	}
}

func TestLookupSeriesByTVDBNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	id, err := c.LookupSeriesByTVDB(context.Background(), "99999")
	if err != nil {
		t.Fatalf("LookupSeriesByTVDB: %v", err)
	}
	if id != 0 {
		t.Fatalf("expected 0 for not found, got %d", id)
	}
}

func TestDeleteSeries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/series/77" {
			t.Errorf("expected path /api/v3/series/77, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("deleteFiles") != "true" {
			t.Errorf("expected deleteFiles=true, got %s", r.URL.Query().Get("deleteFiles"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.DeleteSeries(context.Background(), 77, true); err != nil {
		t.Fatalf("DeleteSeries: %v", err)
	}
}

func TestDeleteSeriesFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Series not found"}`))
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	if err := c.DeleteSeries(context.Background(), 999, true); err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetCalendar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/calendar" {
			t.Errorf("expected path /api/v3/calendar, got %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("start") != "2026-02-08" {
			t.Errorf("expected start=2026-02-08, got %s", q.Get("start"))
		}
		if q.Get("end") != "2026-02-14" {
			t.Errorf("expected end=2026-02-14, got %s", q.Get("end"))
		}
		if q.Get("includeSeries") != "true" {
			t.Errorf("expected includeSeries=true, got %s", q.Get("includeSeries"))
		}
		if q.Get("includeEpisodeImages") != "true" {
			t.Errorf("expected includeEpisodeImages=true, got %s", q.Get("includeEpisodeImages"))
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":            1,
				"seriesId":      10,
				"seasonNumber":  2,
				"episodeNumber": 5,
				"title":         "Test Episode",
				"airDateUtc":    "2026-02-10T20:00:00Z",
				"airDate":       "2026-02-10",
				"hasFile":       false,
				"monitored":     true,
				"series": map[string]any{
					"id":    10,
					"title": "Test Series",
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "test-key")
	data, err := c.GetCalendar(context.Background(), "2026-02-08", "2026-02-14")
	if err != nil {
		t.Fatalf("GetCalendar: %v", err)
	}

	var episodes []struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &episodes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 episode, got %d", len(episodes))
	}
	if episodes[0].Title != "Test Episode" {
		t.Fatalf("expected Test Episode, got %s", episodes[0].Title)
	}
}


