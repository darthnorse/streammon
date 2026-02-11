package embybase

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"streammon/internal/models"
)

func wrapInItemsArray(data []byte) []byte {
	result := append([]byte(`{"Items":[`), data...)
	return append(result, []byte(`]}`)...)
}

func TestGetSessions(t *testing.T) {
	data, err := os.ReadFile("testdata/sessions.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
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

	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (idle excluded), got %d", len(sessions))
	}

	s := sessions[0]
	if s.SessionID != "sess1" {
		t.Errorf("session id = %q, want sess1", s.SessionID)
	}
	if s.UserName != "alice" {
		t.Errorf("user = %q, want alice", s.UserName)
	}
	if s.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", s.MediaType)
	}
	if s.Title != "Inception" {
		t.Errorf("title = %q, want Inception", s.Title)
	}
	if s.DurationMs != 8880000 {
		t.Errorf("duration = %d, want 8880000", s.DurationMs)
	}
	if s.ProgressMs != 3600000 {
		t.Errorf("progress = %d, want 3600000", s.ProgressMs)
	}
	if s.VideoCodec != "h264" {
		t.Errorf("video codec = %q, want h264", s.VideoCodec)
	}
	if s.AudioCodec != "aac" {
		t.Errorf("audio codec = %q, want aac", s.AudioCodec)
	}
	if s.AudioChannels != 6 {
		t.Errorf("audio channels = %d, want 6", s.AudioChannels)
	}
	if s.Container != "mkv" {
		t.Errorf("container = %q, want mkv", s.Container)
	}
	if s.SubtitleCodec != "srt" {
		t.Errorf("subtitle codec = %q, want srt", s.SubtitleCodec)
	}
	if s.VideoDecision != models.TranscodeDecisionTranscode {
		t.Errorf("video decision = %q, want transcode", s.VideoDecision)
	}
	if s.AudioDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("audio decision = %q, want direct play", s.AudioDecision)
	}
	if !s.TranscodeHWDecode {
		t.Error("expected HW decode true (vaapi)")
	}
	if !s.TranscodeHWEncode {
		t.Error("expected HW encode true (vaapi)")
	}
	if s.TranscodeProgress != 55.2 {
		t.Errorf("transcode progress = %f, want 55.2", s.TranscodeProgress)
	}
	if s.VideoResolution != "1080p" {
		t.Errorf("source resolution = %q, want 1080p", s.VideoResolution)
	}
	if s.TranscodeVideoResolution != "720p" {
		t.Errorf("transcode resolution = %q, want 720p", s.TranscodeVideoResolution)
	}

	s2 := sessions[1]
	if s2.MediaType != models.MediaTypeTV {
		t.Errorf("session 2 media type = %q, want episode", s2.MediaType)
	}
	if s2.GrandparentTitle != "Breaking Bad" {
		t.Errorf("grandparent = %q, want Breaking Bad", s2.GrandparentTitle)
	}
	if s2.VideoDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("s2 video decision = %q, want direct play", s2.VideoDecision)
	}
}

func TestTestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Emby-Token"); got != "tok" {
			t.Errorf("expected X-Emby-Token=tok, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTestConnectionAuthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "bad"}, models.ServerTypeEmby)
	err := c.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected auth failure message, got: %s", err.Error())
	}
}

func TestEmptySessionsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestMediaTypeMappings(t *testing.T) {
	tests := []struct {
		embyType string
		want     models.MediaType
	}{
		{"Movie", models.MediaTypeMovie},
		{"Episode", models.MediaTypeTV},
		{"Audio", models.MediaTypeMusic},
		{"TvChannel", models.MediaTypeLiveTV},
		{"AudioBook", models.MediaTypeAudiobook},
		{"Book", models.MediaTypeBook},
		{"MusicVideo", models.MediaTypeMovie},
		{"Video", models.MediaTypeMovie},
	}
	for _, tt := range tests {
		t.Run(tt.embyType, func(t *testing.T) {
			data := fmt.Sprintf(`[{"Id":"s1","UserName":"u","Client":"c","DeviceName":"d","RemoteEndPoint":"1.2.3.4",
				"NowPlayingItem":{"Name":"Test","Type":"%s","RunTimeTicks":100000000},"PlayState":{"PositionTicks":0}}]`, tt.embyType)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(data))
			}))
			defer ts.Close()

			c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
			sessions, err := c.GetSessions(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(sessions) != 1 {
				t.Fatalf("expected 1 session, got %d", len(sessions))
			}
			if sessions[0].MediaType != tt.want {
				t.Errorf("got %q, want %q", sessions[0].MediaType, tt.want)
			}
		})
	}
}

func TestIdleSessionsExcluded(t *testing.T) {
	data := `[{"Id":"idle","UserName":"x","Client":"c","DeviceName":"d","RemoteEndPoint":"1.2.3.4","NowPlayingItem":null}]`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(data))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestGetRecentlyAdded(t *testing.T) {
	data, err := os.ReadFile("testdata/recently_added.json")
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
		if r.URL.Query().Get("SortBy") != "DateCreated" {
			t.Error("missing SortBy=DateCreated query param")
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

	items, err := c.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	item := items[0]
	if item.ItemID != "item1" {
		t.Errorf("item id = %q, want item1", item.ItemID)
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
	if item.ThumbURL != "item1" {
		t.Errorf("thumb url = %q, want item1", item.ThumbURL)
	}
	if item.ServerID != 1 {
		t.Errorf("server id = %d, want 1", item.ServerID)
	}
	if item.ServerName != "TestEmby" {
		t.Errorf("server name = %q, want TestEmby", item.ServerName)
	}
	if item.ServerType != models.ServerTypeEmby {
		t.Errorf("server type = %q, want emby", item.ServerType)
	}

	item2 := items[1]
	if item2.Title != "Breaking Bad - Ozymandias" {
		t.Errorf("title = %q, want Breaking Bad - Ozymandias", item2.Title)
	}
	if item2.MediaType != models.MediaTypeTV {
		t.Errorf("media type = %q, want episode", item2.MediaType)
	}
	if item2.SeasonNumber != 5 {
		t.Errorf("season number = %d, want 5", item2.SeasonNumber)
	}
	if item2.EpisodeNumber != 14 {
		t.Errorf("episode number = %d, want 14", item2.EpisodeNumber)
	}

	item3 := items[2]
	if item3.ThumbURL != "" {
		t.Errorf("thumb url = %q, want empty (no Primary tag)", item3.ThumbURL)
	}
}

func TestGetRecentlyAddedEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items":[]}`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	items, err := c.GetRecentlyAdded(context.Background(), 10)
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

	c := New(models.Server{URL: ts.URL, APIKey: "bad"}, models.ServerTypeEmby)
	_, err := c.GetRecentlyAdded(context.Background(), 10)
	if err == nil {
		t.Error("expected error for 401")
	}
}

func TestGetItemDetails_Movie(t *testing.T) {
	data, err := os.ReadFile("testdata/item_details.json")
	if err != nil {
		t.Fatal(err)
	}
	wrappedData := wrapInItemsArray(data)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("Ids") != "item1" {
			t.Errorf("unexpected Ids param: %s", r.URL.Query().Get("Ids"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(wrappedData)
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	details, err := c.GetItemDetails(context.Background(), "item1")
	if err != nil {
		t.Fatal(err)
	}

	if details.ID != "item1" {
		t.Errorf("id = %q, want item1", details.ID)
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
	if details.Cast[0].ThumbURL != "/api/servers/1/thumb/actor1" {
		t.Errorf("cast[0].thumb_url = %q, want /api/servers/1/thumb/actor1", details.Cast[0].ThumbURL)
	}
	if details.Cast[1].ThumbURL != "/api/servers/1/thumb/actor2" {
		t.Errorf("cast[1].thumb_url = %q, want /api/servers/1/thumb/actor2", details.Cast[1].ThumbURL)
	}
	if details.Cast[2].ThumbURL != "" {
		t.Errorf("cast[2].thumb_url = %q, want empty (no image tag)", details.Cast[2].ThumbURL)
	}

	if details.ServerID != 1 {
		t.Errorf("server id = %d, want 1", details.ServerID)
	}
	if details.ServerName != "TestEmby" {
		t.Errorf("server name = %q, want TestEmby", details.ServerName)
	}
	if details.ServerType != models.ServerTypeEmby {
		t.Errorf("server type = %q, want emby", details.ServerType)
	}
}

func TestGetItemDetails_Episode(t *testing.T) {
	data, err := os.ReadFile("testdata/item_details_episode.json")
	if err != nil {
		t.Fatal(err)
	}
	wrappedData := wrapInItemsArray(data)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(wrappedData)
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, Name: "TestEmby", URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)

	details, err := c.GetItemDetails(context.Background(), "item2")
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

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	_, err := c.GetItemDetails(context.Background(), "nonexistent")
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetItemDetails_EmptyItems(t *testing.T) {
	// Test that 200 OK with empty Items array returns ErrNotFound
	// This can happen when item ID doesn't exist but API returns success
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	_, err := c.GetItemDetails(context.Background(), "nonexistent")
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound for empty Items array, got %v", err)
	}
}

func TestGetLibraries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}

		switch r.URL.Path {
		case "/Library/VirtualFolders":
			w.Write([]byte(`[
				{"Name": "Movies", "CollectionType": "movies", "ItemId": "lib1"},
				{"Name": "TV Shows", "CollectionType": "tvshows", "ItemId": "lib2"},
				{"Name": "Music", "CollectionType": "music", "ItemId": "lib3"}
			]`))
		case "/Items":
			parentID := r.URL.Query().Get("ParentId")
			itemTypes := r.URL.Query().Get("IncludeItemTypes")

			switch parentID {
			case "lib1":
				if itemTypes == "Movie" {
					w.Write([]byte(`{"TotalRecordCount": 150}`))
				} else {
					w.Write([]byte(`{"TotalRecordCount": 0}`))
				}
			case "lib2":
				switch itemTypes {
				case "Series":
					w.Write([]byte(`{"TotalRecordCount": 50}`))
				case "Season":
					w.Write([]byte(`{"TotalRecordCount": 200}`))
				case "Episode":
					w.Write([]byte(`{"TotalRecordCount": 2500}`))
				default:
					w.Write([]byte(`{"TotalRecordCount": 0}`))
				}
			case "lib3":
				switch itemTypes {
				case "MusicArtist":
					w.Write([]byte(`{"TotalRecordCount": 100}`))
				case "MusicAlbum":
					w.Write([]byte(`{"TotalRecordCount": 500}`))
				case "Audio":
					w.Write([]byte(`{"TotalRecordCount": 5000}`))
				default:
					w.Write([]byte(`{"TotalRecordCount": 0}`))
				}
			default:
				w.Write([]byte(`{"TotalRecordCount": 0}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	libs, err := c.GetLibraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(libs) != 3 {
		t.Fatalf("expected 3 libraries, got %d", len(libs))
	}

	// Movies library
	movies := libs[0]
	if movies.ID != "lib1" {
		t.Errorf("movies id = %q, want lib1", movies.ID)
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
	if movies.ServerID != 1 {
		t.Errorf("movies server_id = %d, want 1", movies.ServerID)
	}
	if movies.ServerName != "TestEmby" {
		t.Errorf("movies server_name = %q, want TestEmby", movies.ServerName)
	}
	if movies.ServerType != models.ServerTypeEmby {
		t.Errorf("movies server_type = %q, want emby", movies.ServerType)
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
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	libs, err := c.GetLibraries(context.Background())
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

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	_, err := c.GetLibraries(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestGetLibrariesCountError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Library/VirtualFolders":
			w.Write([]byte(`[{"Name": "Movies", "CollectionType": "movies", "ItemId": "lib1"}]`))
		case "/Items":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	libs, err := c.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
	if libs[0].Name != "Movies" {
		t.Errorf("expected name Movies, got %s", libs[0].Name)
	}
	if libs[0].ItemCount != 0 {
		t.Errorf("expected 0 item count on count failure, got %d", libs[0].ItemCount)
	}
}

func TestGetLibrariesOtherType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Library/VirtualFolders":
			w.Write([]byte(`[{"Name": "Photos", "CollectionType": "photos", "ItemId": "lib1"}]`))
		case "/Items":
			// For unknown types, should request without IncludeItemTypes filter
			if r.URL.Query().Get("IncludeItemTypes") != "" {
				t.Errorf("expected no IncludeItemTypes for unknown type, got %q", r.URL.Query().Get("IncludeItemTypes"))
			}
			w.Write([]byte(`{"TotalRecordCount": 42}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	libs, err := c.GetLibraries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
	if libs[0].Type != models.LibraryTypeOther {
		t.Errorf("type = %q, want other", libs[0].Type)
	}
	if libs[0].ItemCount != 42 {
		t.Errorf("item_count = %d, want 42", libs[0].ItemCount)
	}
}

func TestGetLibraryItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("Recursive") != "true" {
			t.Error("missing Recursive=true")
		}

		parentID := r.URL.Query().Get("ParentId")
		itemType := r.URL.Query().Get("IncludeItemTypes")

		switch {
		case parentID == "lib1" && itemType == "Movie":
			w.Write([]byte(`{
				"Items": [
					{"Id": "movie1", "Name": "Inception", "Type": "Movie", "ProductionYear": 2010, "DateCreated": "2024-01-15T10:30:00Z", "RecursiveItemCount": 0}
				],
				"TotalRecordCount": 1
			}`))
		case parentID == "lib1" && itemType == "Series":
			w.Write([]byte(`{
				"Items": [
					{"Id": "series1", "Name": "Breaking Bad", "Type": "Series", "ProductionYear": 2008, "DateCreated": "2024-01-10T08:00:00Z", "RecursiveItemCount": 62}
				],
				"TotalRecordCount": 1
			}`))
		case parentID == "series1" && itemType == "Episode":
			w.Write([]byte(`{
				"Items": [
					{"Id": "ep1", "Name": "Pilot", "Type": "Episode", "MediaSources": [{"Size": 1000000000}]},
					{"Id": "ep2", "Name": "Cat's in the Bag", "Type": "Episode", "MediaSources": [{"Size": 1500000000}]}
				],
				"TotalRecordCount": 2
			}`))
		default:
			t.Errorf("unexpected request: ParentId=%s, IncludeItemTypes=%s", parentID, itemType)
			w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
		}
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	items, err := c.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items (1 movie + 1 series), got %d", len(items))
	}

	// First item should be the movie
	movie := items[0]
	if movie.ItemID != "movie1" {
		t.Errorf("movie id = %q, want movie1", movie.ItemID)
	}
	if movie.Title != "Inception" {
		t.Errorf("movie title = %q, want Inception", movie.Title)
	}
	if movie.MediaType != models.MediaTypeMovie {
		t.Errorf("movie media type = %q, want movie", movie.MediaType)
	}
	if movie.Year != 2010 {
		t.Errorf("movie year = %d, want 2010", movie.Year)
	}
	if movie.ServerID != 1 {
		t.Errorf("movie server id = %d, want 1", movie.ServerID)
	}
	if movie.LibraryID != "lib1" {
		t.Errorf("movie library id = %q, want lib1", movie.LibraryID)
	}

	// Second item should be the series
	series := items[1]
	if series.ItemID != "series1" {
		t.Errorf("series id = %q, want series1", series.ItemID)
	}
	if series.Title != "Breaking Bad" {
		t.Errorf("series title = %q, want Breaking Bad", series.Title)
	}
	if series.MediaType != models.MediaTypeTV {
		t.Errorf("series media type = %q, want episode", series.MediaType)
	}
	if series.EpisodeCount != 62 {
		t.Errorf("series episode count = %d, want 62", series.EpisodeCount)
	}
	// File size should be sum of episode sizes (1GB + 1.5GB = 2.5GB)
	if series.FileSize != 2500000000 {
		t.Errorf("series file size = %d, want 2500000000", series.FileSize)
	}
}

func TestGetLibraryItemsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	items, err := c.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGetLibraryItemsPagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemType := r.URL.Query().Get("IncludeItemTypes")
		startIndex := r.URL.Query().Get("StartIndex")

		if itemType == "Movie" {
			callCount++
			switch startIndex {
			case "0":
				// First batch: return 100 items, total 150
				items := make([]string, 100)
				for i := 0; i < 100; i++ {
					items[i] = fmt.Sprintf(`{"Id": "movie%d", "Name": "Movie %d", "Type": "Movie", "ProductionYear": 2020, "DateCreated": "2024-01-01T00:00:00Z"}`, i, i)
				}
				w.Write([]byte(fmt.Sprintf(`{"Items": [%s], "TotalRecordCount": 150}`, strings.Join(items, ","))))
			case "100":
				// Second batch: return remaining 50 items
				items := make([]string, 50)
				for i := 0; i < 50; i++ {
					items[i] = fmt.Sprintf(`{"Id": "movie%d", "Name": "Movie %d", "Type": "Movie", "ProductionYear": 2020, "DateCreated": "2024-01-01T00:00:00Z"}`, 100+i, 100+i)
				}
				w.Write([]byte(fmt.Sprintf(`{"Items": [%s], "TotalRecordCount": 150}`, strings.Join(items, ","))))
			default:
				t.Errorf("unexpected StartIndex for movies: %s", startIndex)
				w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
			}
		} else if itemType == "Series" {
			// No series in this library
			w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	items, err := c.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 150 {
		t.Errorf("expected 150 items, got %d", len(items))
	}

	// Should have made 2 calls for movies (pagination)
	if callCount != 2 {
		t.Errorf("expected 2 movie batch calls, got %d", callCount)
	}
}

func TestGetLibraryItemsContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a batch that would trigger pagination
		w.Write([]byte(`{"Items": [{"Id": "m1", "Name": "Movie", "Type": "Movie", "ProductionYear": 2020, "DateCreated": "2024-01-01T00:00:00Z"}], "TotalRecordCount": 1000}`))
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.GetLibraryItems(ctx, "lib1")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestGetLibraryItemsMovieError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemType := r.URL.Query().Get("IncludeItemTypes")
		if itemType == "Movie" {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	_, err := c.GetLibraryItems(context.Background(), "lib1")
	if err == nil {
		t.Error("expected error when movie fetch fails")
	}
	// Check error wrapping
	if !strings.Contains(err.Error(), "fetch movies") {
		t.Errorf("error should contain 'fetch movies': %v", err)
	}
}

func TestGetLibraryItemsSeriesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemType := r.URL.Query().Get("IncludeItemTypes")
		if itemType == "Movie" {
			w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
		} else if itemType == "Series" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	_, err := c.GetLibraryItems(context.Background(), "lib1")
	if err == nil {
		t.Error("expected error when series fetch fails")
	}
	// Check error wrapping
	if !strings.Contains(err.Error(), "fetch series") {
		t.Errorf("error should contain 'fetch series': %v", err)
	}
}

func TestGetLibraryItemsWithMediaInfo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemType := r.URL.Query().Get("IncludeItemTypes")
		if itemType == "Movie" {
			w.Write([]byte(`{
				"Items": [{
					"Id": "movie1",
					"Name": "4K Movie",
					"Type": "Movie",
					"ProductionYear": 2023,
					"DateCreated": "2024-01-15T10:30:00Z",
					"MediaSources": [{
						"Size": 50000000000,
						"MediaStreams": [
							{"Type": "Video", "Height": 2160, "Width": 3840},
							{"Type": "Audio", "Channels": 8}
						]
					}]
				}],
				"TotalRecordCount": 1
			}`))
		} else {
			w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	items, err := c.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.VideoResolution != "4K" {
		t.Errorf("resolution = %q, want 4K", item.VideoResolution)
	}
	if item.FileSize != 50000000000 {
		t.Errorf("file size = %d, want 50000000000", item.FileSize)
	}
}

func TestDeriveDynamicRange(t *testing.T) {
	tests := []struct {
		name   string
		stream mediaStream
		want   string
	}{
		{
			name:   "SDR default",
			stream: mediaStream{},
			want:   "SDR",
		},
		{
			name:   "Dolby Vision DOVI",
			stream: mediaStream{VideoRangeType: "DOVI"},
			want:   "Dolby Vision",
		},
		{
			name:   "Dolby Vision with HDR10",
			stream: mediaStream{VideoRangeType: "DOVIWithHDR10"},
			want:   "Dolby Vision",
		},
		{
			name:   "Dolby Vision with HLG",
			stream: mediaStream{VideoRangeType: "DOVIWithHLG"},
			want:   "Dolby Vision",
		},
		{
			name:   "Dolby Vision with SDR",
			stream: mediaStream{VideoRangeType: "DOVIWithSDR"},
			want:   "Dolby Vision",
		},
		{
			name:   "HDR10",
			stream: mediaStream{VideoRangeType: "HDR10"},
			want:   "HDR10",
		},
		{
			name:   "HDR10+",
			stream: mediaStream{VideoRangeType: "HDR10+"},
			want:   "HDR10+",
		},
		{
			name:   "HLG",
			stream: mediaStream{VideoRangeType: "HLG"},
			want:   "HLG",
		},
		{
			name:   "Generic HDR via VideoRange",
			stream: mediaStream{VideoRange: "HDR"},
			want:   "HDR",
		},
		{
			name:   "HDR via VideoRange with bit depth",
			stream: mediaStream{VideoRange: "HDR", BitDepth: 10},
			want:   "HDR",
		},
		{
			name:   "SDR explicit",
			stream: mediaStream{VideoRange: "SDR"},
			want:   "SDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveDynamicRange(tt.stream)
			if got != tt.want {
				t.Errorf("deriveDynamicRange() = %q, want %q", got, tt.want)
			}
		})
	}
}


func TestDeleteItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing auth header")
		}
		if r.URL.Path != "/Items/12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.DeleteItem(context.Background(), "12345")
	if err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestDeleteItemError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.DeleteItem(context.Background(), "12345")
	if err == nil {
		t.Error("expected error for 403")
	}
}

func TestDeleteItemErrorIncludesBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Access to the path is denied"))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "test-key"}, models.ServerTypeJellyfin)
	err := c.DeleteItem(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	want := "jellyfin delete: status 500: Access to the path is denied"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
