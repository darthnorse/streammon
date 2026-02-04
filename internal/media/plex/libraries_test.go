package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestGetLibraries(t *testing.T) {
	sectionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Directory key="1" title="Movies" type="movie"/>
  <Directory key="2" title="TV Shows" type="show"/>
  <Directory key="3" title="Music" type="artist"/>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(sectionsXML))
		case "/library/sections/1/all":
			// Movies - type=1
			if r.URL.Query().Get("type") == "1" {
				w.Write([]byte(`<MediaContainer totalSize="150"/>`))
			} else {
				w.Write([]byte(`<MediaContainer size="0"/>`))
			}
		case "/library/sections/2/all":
			// TV Shows - type=2 (shows), type=3 (seasons), type=4 (episodes)
			switch r.URL.Query().Get("type") {
			case "2":
				w.Write([]byte(`<MediaContainer totalSize="50"/>`))
			case "3":
				w.Write([]byte(`<MediaContainer totalSize="200"/>`))
			case "4":
				w.Write([]byte(`<MediaContainer totalSize="2500"/>`))
			default:
				w.Write([]byte(`<MediaContainer size="0"/>`))
			}
		case "/library/sections/3/all":
			// Music - type=8 (artists), type=9 (albums), type=10 (tracks)
			switch r.URL.Query().Get("type") {
			case "8":
				w.Write([]byte(`<MediaContainer totalSize="100"/>`))
			case "9":
				w.Write([]byte(`<MediaContainer totalSize="500"/>`))
			case "10":
				w.Write([]byte(`<MediaContainer totalSize="5000"/>`))
			default:
				w.Write([]byte(`<MediaContainer size="0"/>`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{
		ID:     1,
		Name:   "TestPlex",
		Type:   models.ServerTypePlex,
		URL:    ts.URL,
		APIKey: "test-token",
	})

	libs, err := srv.GetLibraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(libs) != 3 {
		t.Fatalf("expected 3 libraries, got %d", len(libs))
	}

	// Movies library
	movies := libs[0]
	if movies.ID != "1" {
		t.Errorf("movies id = %q, want 1", movies.ID)
	}
	if movies.Name != "Movies" {
		t.Errorf("movies name = %q, want Movies", movies.Name)
	}
	if movies.Type != models.LibraryTypeMovie {
		t.Errorf("movies type = %q, want movie", movies.Type)
	}
	if movies.ItemCount != 150 {
		t.Errorf("movies item_count = %d, want 150", movies.ItemCount)
	}
	if movies.ChildCount != 0 {
		t.Errorf("movies child_count = %d, want 0", movies.ChildCount)
	}
	if movies.GrandchildCount != 0 {
		t.Errorf("movies grandchild_count = %d, want 0", movies.GrandchildCount)
	}
	if movies.ServerID != 1 {
		t.Errorf("movies server_id = %d, want 1", movies.ServerID)
	}
	if movies.ServerName != "TestPlex" {
		t.Errorf("movies server_name = %q, want TestPlex", movies.ServerName)
	}
	if movies.ServerType != models.ServerTypePlex {
		t.Errorf("movies server_type = %q, want plex", movies.ServerType)
	}

	// TV Shows library
	tv := libs[1]
	if tv.Name != "TV Shows" {
		t.Errorf("tv name = %q, want TV Shows", tv.Name)
	}
	if tv.Type != models.LibraryTypeShow {
		t.Errorf("tv type = %q, want show", tv.Type)
	}
	if tv.ItemCount != 50 {
		t.Errorf("tv item_count = %d, want 50", tv.ItemCount)
	}
	if tv.ChildCount != 200 {
		t.Errorf("tv child_count = %d, want 200", tv.ChildCount)
	}
	if tv.GrandchildCount != 2500 {
		t.Errorf("tv grandchild_count = %d, want 2500", tv.GrandchildCount)
	}

	// Music library
	music := libs[2]
	if music.Name != "Music" {
		t.Errorf("music name = %q, want Music", music.Name)
	}
	if music.Type != models.LibraryTypeMusic {
		t.Errorf("music type = %q, want music", music.Type)
	}
	if music.ItemCount != 100 {
		t.Errorf("music item_count = %d, want 100", music.ItemCount)
	}
	if music.ChildCount != 500 {
		t.Errorf("music child_count = %d, want 500", music.ChildCount)
	}
	if music.GrandchildCount != 5000 {
		t.Errorf("music grandchild_count = %d, want 5000", music.GrandchildCount)
	}
}

func TestGetLibrariesEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<MediaContainer/>`))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	libs, err := srv.GetLibraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 0 {
		t.Errorf("expected 0 libraries, got %d", len(libs))
	}
}

func TestGetLibrariesServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	_, err := srv.GetLibraries(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestGetLibrariesUsesSize(t *testing.T) {
	sectionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Directory key="1" title="Movies" type="movie"/>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(sectionsXML))
		case "/library/sections/1/all":
			// No totalSize, should fall back to size
			w.Write([]byte(`<MediaContainer size="42"/>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	libs, err := srv.GetLibraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
	if libs[0].ItemCount != 42 {
		t.Errorf("item_count = %d, want 42 (from size attr)", libs[0].ItemCount)
	}
}
