package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetRecentlyAdded(t *testing.T) {
	data, err := os.ReadFile("testdata/recently_added.xml")
	if err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/hubs/home/recentlyAdded" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		typeParam := r.URL.Query().Get("type")
		if typeParam != "1" && typeParam != "2" {
			t.Errorf("unexpected type parameter: %s", typeParam)
		}
		requestCount++
		w.Write(data)
	}))
	defer ts.Close()

	srv := New(models.Server{
		ID:     1,
		Name:   "TestPlex",
		Type:   models.ServerTypePlex,
		URL:    ts.URL,
		APIKey: "test-token",
	})

	items, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests (movies + shows), got %d", requestCount)
	}

	// 3 items per type × 2 types = 6 total
	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(items))
	}

	item := items[0]
	if item.ItemID != "12345" {
		t.Errorf("item id = %q, want 12345", item.ItemID)
	}
	if item.Title != "Oppenheimer" {
		t.Errorf("title = %q, want Oppenheimer", item.Title)
	}
	if item.Year != 2023 {
		t.Errorf("year = %d, want 2023", item.Year)
	}
	if item.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", item.MediaType)
	}
	if item.ThumbURL != "12345" {
		t.Errorf("thumb url = %q, want 12345", item.ThumbURL)
	}
	if item.ServerID != 1 {
		t.Errorf("server id = %d, want 1", item.ServerID)
	}
	if item.ServerName != "TestPlex" {
		t.Errorf("server name = %q, want TestPlex", item.ServerName)
	}
	if item.ServerType != models.ServerTypePlex {
		t.Errorf("server type = %q, want plex", item.ServerType)
	}

	// Verify episode uses series poster (grandparentRatingKey) for thumb
	hasEpisode := false
	for _, it := range items {
		if it.Title == "Breaking Bad - Ozymandias" {
			hasEpisode = true
			if it.MediaType != models.MediaTypeTV {
				t.Errorf("episode media type = %q, want episode", it.MediaType)
			}
			if it.ThumbURL != "55555" {
				t.Errorf("episode thumb = %q, want 55555 (series grandparentRatingKey)", it.ThumbURL)
			}
			break
		}
	}
	if !hasEpisode {
		t.Error("expected to find episode 'Breaking Bad - Ozymandias'")
	}
}

func TestGetRecentlyAddedEmpty(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><MediaContainer size="0"></MediaContainer>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGetRecentlyAddedError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "bad"})
	items, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Errorf("expected no error (errors logged, not returned), got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items on error, got %d", len(items))
	}
}
