package embybase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestGetSeasons(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("ParentId") != "show1" {
			t.Errorf("unexpected ParentId: %s", r.URL.Query().Get("ParentId"))
		}
		if r.URL.Query().Get("IncludeItemTypes") != "Season" {
			t.Errorf("unexpected IncludeItemTypes: %s", r.URL.Query().Get("IncludeItemTypes"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"Items": [
				{"Id": "season0", "Name": "Specials", "IndexNumber": 0},
				{"Id": "season1", "Name": "Season 1", "IndexNumber": 1},
				{"Id": "season2", "Name": "Season 2", "IndexNumber": 2}
			]
		}`))
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	seasons, err := c.GetSeasons(context.Background(), "show1")
	if err != nil {
		t.Fatal(err)
	}

	if len(seasons) != 3 {
		t.Fatalf("expected 3 seasons, got %d", len(seasons))
	}

	if seasons[0].ID != "season0" {
		t.Errorf("season[0].ID = %q, want season0", seasons[0].ID)
	}
	if seasons[0].Number != 0 {
		t.Errorf("season[0].Number = %d, want 0", seasons[0].Number)
	}
	if seasons[0].Title != "Specials" {
		t.Errorf("season[0].Title = %q, want Specials", seasons[0].Title)
	}

	if seasons[1].ID != "season1" {
		t.Errorf("season[1].ID = %q, want season1", seasons[1].ID)
	}
	if seasons[1].Number != 1 {
		t.Errorf("season[1].Number = %d, want 1", seasons[1].Number)
	}
	if seasons[1].Title != "Season 1" {
		t.Errorf("season[1].Title = %q, want Season 1", seasons[1].Title)
	}

	if seasons[2].ID != "season2" {
		t.Errorf("season[2].ID = %q, want season2", seasons[2].ID)
	}
	if seasons[2].Number != 2 {
		t.Errorf("season[2].Number = %d, want 2", seasons[2].Number)
	}
	if seasons[2].Title != "Season 2" {
		t.Errorf("season[2].Title = %q, want Season 2", seasons[2].Title)
	}
}

func TestGetSeasonsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items": []}`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	seasons, err := c.GetSeasons(context.Background(), "show1")
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 0 {
		t.Errorf("expected 0 seasons, got %d", len(seasons))
	}
}
