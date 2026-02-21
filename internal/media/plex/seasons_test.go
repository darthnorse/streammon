package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestGetSeasons(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Directory ratingKey="100" index="0" title="Specials" />
  <Directory ratingKey="101" index="1" title="Season 1" />
  <Directory ratingKey="102" index="2" title="Season 2" />
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
			t.Error("missing auth header")
		}
		if r.URL.Path != "/library/metadata/555/children" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	seasons, err := srv.GetSeasons(context.Background(), "555")
	if err != nil {
		t.Fatal(err)
	}

	if len(seasons) != 3 {
		t.Fatalf("expected 3 seasons, got %d", len(seasons))
	}

	if seasons[0].ID != "100" {
		t.Errorf("season[0].ID = %q, want 100", seasons[0].ID)
	}
	if seasons[0].Number != 0 {
		t.Errorf("season[0].Number = %d, want 0", seasons[0].Number)
	}
	if seasons[0].Title != "Specials" {
		t.Errorf("season[0].Title = %q, want Specials", seasons[0].Title)
	}

	if seasons[1].ID != "101" {
		t.Errorf("season[1].ID = %q, want 101", seasons[1].ID)
	}
	if seasons[1].Number != 1 {
		t.Errorf("season[1].Number = %d, want 1", seasons[1].Number)
	}
	if seasons[1].Title != "Season 1" {
		t.Errorf("season[1].Title = %q, want Season 1", seasons[1].Title)
	}

	if seasons[2].ID != "102" {
		t.Errorf("season[2].ID = %q, want 102", seasons[2].ID)
	}
	if seasons[2].Number != 2 {
		t.Errorf("season[2].Number = %d, want 2", seasons[2].Number)
	}
	if seasons[2].Title != "Season 2" {
		t.Errorf("season[2].Title = %q, want Season 2", seasons[2].Title)
	}
}

func TestGetSeasonsEmpty(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	seasons, err := srv.GetSeasons(context.Background(), "999")
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 0 {
		t.Errorf("expected 0 seasons, got %d", len(seasons))
	}
}

func TestGetSeasonsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	_, err := srv.GetSeasons(context.Background(), "999")
	if err == nil {
		t.Error("expected error for 500")
	}
}
