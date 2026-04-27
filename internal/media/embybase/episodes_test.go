package embybase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetEpisodes(t *testing.T) {
	data, err := os.ReadFile("testdata/episodes.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("ParentId") != "season-1" {
			t.Errorf("unexpected ParentId: %s", r.URL.Query().Get("ParentId"))
		}
		if r.URL.Query().Get("IncludeItemTypes") != "Episode" {
			t.Errorf("unexpected IncludeItemTypes: %s", r.URL.Query().Get("IncludeItemTypes"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	episodes, err := c.GetEpisodes(context.Background(), "season-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}

	got := episodes[0]
	if got.ID != "ep1" {
		t.Errorf("episodes[0].ID = %q, want ep1", got.ID)
	}
	if got.Number != 1 {
		t.Errorf("episodes[0].Number = %d, want 1", got.Number)
	}
	if got.Title != "Pilot" {
		t.Errorf("episodes[0].Title = %q, want Pilot", got.Title)
	}
	if got.Summary != "The first one." {
		t.Errorf("episodes[0].Summary = %q, want %q", got.Summary, "The first one.")
	}
	if got.DurationMs != 2880000 {
		t.Errorf("episodes[0].DurationMs = %d, want 2880000", got.DurationMs)
	}
	if got.AirDate != "2020-01-15" {
		t.Errorf("episodes[0].AirDate = %q, want 2020-01-15", got.AirDate)
	}
	if got.ThumbURL == "" {
		t.Errorf("episodes[0].ThumbURL is empty, want non-empty")
	}

	if episodes[1].ID != "ep2" {
		t.Errorf("episodes[1].ID = %q, want ep2", episodes[1].ID)
	}
	if episodes[1].AirDate != "2020-01-22" {
		t.Errorf("episodes[1].AirDate = %q, want 2020-01-22", episodes[1].AirDate)
	}
}

func TestGetEpisodesEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	episodes, err := c.GetEpisodes(context.Background(), "season-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 0 {
		t.Errorf("expected 0 episodes, got %d", len(episodes))
	}
}
