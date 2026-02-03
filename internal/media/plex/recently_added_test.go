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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/library/recentlyAdded" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
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

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
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

	item2 := items[1]
	if item2.Title != "Breaking Bad - Ozymandias" {
		t.Errorf("title = %q, want Breaking Bad - Ozymandias", item2.Title)
	}
	if item2.MediaType != models.MediaTypeTV {
		t.Errorf("media type = %q, want episode", item2.MediaType)
	}

	item3 := items[2]
	if item3.ThumbURL != "" {
		t.Errorf("thumb url = %q, want empty (no thumb attr)", item3.ThumbURL)
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
	_, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err == nil {
		t.Error("expected error for 401")
	}
}
