package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetItemDetails_Movie(t *testing.T) {
	data, err := os.ReadFile("testdata/item_details.xml")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/library/metadata/12345" {
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

	details, err := srv.GetItemDetails(context.Background(), "12345")
	if err != nil {
		t.Fatal(err)
	}

	if details.ID != "12345" {
		t.Errorf("id = %q, want 12345", details.ID)
	}
	if details.Title != "Oppenheimer" {
		t.Errorf("title = %q, want Oppenheimer", details.Title)
	}
	if details.Year != 2023 {
		t.Errorf("year = %d, want 2023", details.Year)
	}
	if details.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", details.MediaType)
	}
	if details.Rating != 8.5 {
		t.Errorf("rating = %f, want 8.5", details.Rating)
	}
	if details.ContentRating != "R" {
		t.Errorf("content rating = %q, want R", details.ContentRating)
	}
	if details.DurationMs != 10800000 {
		t.Errorf("duration = %d, want 10800000", details.DurationMs)
	}
	if details.Studio != "Universal Pictures" {
		t.Errorf("studio = %q, want Universal Pictures", details.Studio)
	}

	expectedGenres := []string{"Drama", "History", "Biography"}
	if len(details.Genres) != len(expectedGenres) {
		t.Fatalf("genres count = %d, want %d", len(details.Genres), len(expectedGenres))
	}
	for i, g := range expectedGenres {
		if details.Genres[i] != g {
			t.Errorf("genre[%d] = %q, want %q", i, details.Genres[i], g)
		}
	}

	if len(details.Directors) != 1 || details.Directors[0] != "Christopher Nolan" {
		t.Errorf("directors = %v, want [Christopher Nolan]", details.Directors)
	}

	if len(details.Cast) != 3 {
		t.Fatalf("cast count = %d, want 3", len(details.Cast))
	}
	if details.Cast[0].Name != "Cillian Murphy" {
		t.Errorf("cast[0].name = %q, want Cillian Murphy", details.Cast[0].Name)
	}
	if details.Cast[0].Role != "J. Robert Oppenheimer" {
		t.Errorf("cast[0].role = %q, want J. Robert Oppenheimer", details.Cast[0].Role)
	}
	if details.Cast[2].ThumbURL != "" {
		t.Errorf("cast[2].thumb_url = %q, want empty (no thumb)", details.Cast[2].ThumbURL)
	}

	if details.ServerID != 1 {
		t.Errorf("server id = %d, want 1", details.ServerID)
	}
	if details.ServerName != "TestPlex" {
		t.Errorf("server name = %q, want TestPlex", details.ServerName)
	}
	if details.ServerType != models.ServerTypePlex {
		t.Errorf("server type = %q, want plex", details.ServerType)
	}
}

func TestGetItemDetails_Episode(t *testing.T) {
	data, err := os.ReadFile("testdata/item_details_episode.xml")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "tok"})

	details, err := srv.GetItemDetails(context.Background(), "67890")
	if err != nil {
		t.Fatal(err)
	}

	if details.MediaType != models.MediaTypeTV {
		t.Errorf("media type = %q, want episode", details.MediaType)
	}
	if details.SeriesTitle != "Breaking Bad" {
		t.Errorf("series title = %q, want Breaking Bad", details.SeriesTitle)
	}
	if details.SeasonNumber != 5 {
		t.Errorf("season number = %d, want 5", details.SeasonNumber)
	}
	if details.EpisodeNumber != 14 {
		t.Errorf("episode number = %d, want 14", details.EpisodeNumber)
	}
}

func TestGetItemDetails_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	_, err := srv.GetItemDetails(context.Background(), "99999")
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetItemDetails_EmptyContainer(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><MediaContainer size="0"></MediaContainer>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	_, err := srv.GetItemDetails(context.Background(), "12345")
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound for empty container, got %v", err)
	}
}

func TestGetItemDetails_TVSeries(t *testing.T) {
	// TV series metadata is returned as Directory element, not Video
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Directory ratingKey="55555" type="show" title="Breaking Bad" year="2008"
    summary="A high school chemistry teacher turned meth producer."
    thumb="/library/metadata/55555/thumb/123" contentRating="TV-MA" rating="9.5"
    studio="AMC">
    <Genre tag="Drama" />
    <Genre tag="Crime" />
    <Role tag="Bryan Cranston" role="Walter White" />
  </Directory>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "tok"})
	details, err := srv.GetItemDetails(context.Background(), "55555")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if details.ID != "55555" {
		t.Errorf("id = %q, want 55555", details.ID)
	}
	if details.Title != "Breaking Bad" {
		t.Errorf("title = %q, want Breaking Bad", details.Title)
	}
	if details.Year != 2008 {
		t.Errorf("year = %d, want 2008", details.Year)
	}
	if details.MediaType != models.MediaTypeTV {
		t.Errorf("media type = %q, want episode", details.MediaType)
	}
	if len(details.Genres) != 2 || details.Genres[0] != "Drama" {
		t.Errorf("genres = %v, want [Drama, Crime]", details.Genres)
	}
	if details.Studio != "AMC" {
		t.Errorf("studio = %q, want AMC", details.Studio)
	}
}
