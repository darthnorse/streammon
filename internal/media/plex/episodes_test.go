package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetEpisodes(t *testing.T) {
	body, err := os.ReadFile("testdata/episodes.xml")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/library/metadata/season-key/children" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write(body)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	got, err := srv.GetEpisodes(context.Background(), "season-key")
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(got))
	}

	want := models.Episode{
		ID:         "111",
		Number:     1,
		Title:      "Pilot",
		Summary:    "The first one.",
		DurationMs: 3000000,
		AirDate:    "2020-01-15",
		ThumbURL:   "111",
	}
	if got[0] != want {
		t.Errorf("episode[0] = %+v, want %+v", got[0], want)
	}

	if got[1].ID != "222" {
		t.Errorf("episode[1].ID = %q, want 222", got[1].ID)
	}
	if got[1].Number != 2 {
		t.Errorf("episode[1].Number = %d, want 2", got[1].Number)
	}
	if got[1].Title != "The Second" {
		t.Errorf("episode[1].Title = %q, want The Second", got[1].Title)
	}
	if got[1].DurationMs != 2700000 {
		t.Errorf("episode[1].DurationMs = %d, want 2700000", got[1].DurationMs)
	}
	if got[1].AirDate != "2020-01-22" {
		t.Errorf("episode[1].AirDate = %q, want 2020-01-22", got[1].AirDate)
	}
	if got[1].ThumbURL != "222" {
		t.Errorf("episode[1].ThumbURL = %q, want 222", got[1].ThumbURL)
	}
}
