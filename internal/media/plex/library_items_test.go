package plex

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/mediautil"
	"streammon/internal/models"
)

// newShowHistoryServer creates an httptest.Server for tests that need shows + history.
// moviesXML/showsXML/historyHandler are the custom parts; everything else returns empty XML.
func newShowHistoryServer(t *testing.T, showsXML string, historyHandler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeMovie:
			w.Write([]byte(`<MediaContainer totalSize="0"/>`))
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeShow:
			w.Write([]byte(showsXML))
		case r.URL.Path == "/status/sessions/history/all":
			historyHandler(w, r)
		default:
			w.Write([]byte(`<MediaContainer totalSize="0"/>`))
		}
	}))
}

func TestGetLibraryItems(t *testing.T) {
	moviesXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Video ratingKey="100" type="movie" title="Inception" year="2010" addedAt="1700000000" lastViewedAt="1700100000">
    <Media videoResolution="1080"><Part size="5000000000"/></Media>
  </Video>
</MediaContainer>`

	showsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Directory ratingKey="200" type="show" title="Breaking Bad" year="2008" addedAt="1700000000" leafCount="62"/>
</MediaContainer>`

	episodesXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="2">
  <Video ratingKey="201" type="episode">
    <Media><Part size="1000000000"/></Media>
  </Video>
  <Video ratingKey="202" type="episode">
    <Media><Part size="1500000000"/></Media>
  </Video>
</MediaContainer>`

	historyXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Video grandparentRatingKey="200" viewedAt="1700500000"/>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeMovie:
			w.Write([]byte(moviesXML))
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeShow:
			w.Write([]byte(showsXML))
		case r.URL.Path == "/library/metadata/200/allLeaves":
			w.Write([]byte(episodesXML))
		case r.URL.Path == "/status/sessions/history/all":
			w.Write([]byte(historyXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	movie := items[0]
	if movie.Title != "Inception" {
		t.Errorf("movie title = %q, want Inception", movie.Title)
	}
	if movie.LastWatchedAt == nil {
		t.Error("movie LastWatchedAt should not be nil")
	}

	show := items[1]
	if show.Title != "Breaking Bad" {
		t.Errorf("show title = %q, want Breaking Bad", show.Title)
	}
	if show.FileSize != 2500000000 {
		t.Errorf("show file size = %d, want 2500000000", show.FileSize)
	}
	if show.LastWatchedAt == nil {
		t.Fatal("show LastWatchedAt should not be nil")
	}
	want := time.Unix(1700500000, 0).UTC()
	if !show.LastWatchedAt.Equal(want) {
		t.Errorf("show LastWatchedAt = %v, want %v", *show.LastWatchedAt, want)
	}
}

func TestShowsEnrichedFromHistory(t *testing.T) {
	showsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="2">
  <Directory ratingKey="300" type="show" title="NullShow" year="2020" addedAt="1700000000" leafCount="10"/>
  <Directory ratingKey="400" type="show" title="StaleShow" year="2019" addedAt="1700000000" lastViewedAt="1600000000" leafCount="5"/>
</MediaContainer>`

	historyXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="2">
  <Video grandparentRatingKey="300" viewedAt="1700200000"/>
  <Video grandparentRatingKey="400" viewedAt="1700300000"/>
</MediaContainer>`

	ts := newShowHistoryServer(t, showsXML, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(historyXML))
	})
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	nullShow := items[0]
	if nullShow.LastWatchedAt == nil {
		t.Fatal("NullShow LastWatchedAt should be set from history")
	}
	if !nullShow.LastWatchedAt.Equal(time.Unix(1700200000, 0).UTC()) {
		t.Errorf("NullShow LastWatchedAt = %v, want %v", *nullShow.LastWatchedAt, time.Unix(1700200000, 0).UTC())
	}

	staleShow := items[1]
	if staleShow.LastWatchedAt == nil {
		t.Fatal("StaleShow LastWatchedAt should be set")
	}
	if !staleShow.LastWatchedAt.Equal(time.Unix(1700300000, 0).UTC()) {
		t.Errorf("StaleShow LastWatchedAt = %v, want %v", *staleShow.LastWatchedAt, time.Unix(1700300000, 0).UTC())
	}
}

func TestHistoryNeverDowngrades(t *testing.T) {
	showsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Directory ratingKey="500" type="show" title="NewerShow" year="2021" addedAt="1700000000" lastViewedAt="1800000000" leafCount="8"/>
</MediaContainer>`

	historyXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Video grandparentRatingKey="500" viewedAt="1700000000"/>
</MediaContainer>`

	ts := newShowHistoryServer(t, showsXML, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(historyXML))
	})
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	show := items[0]
	if show.LastWatchedAt == nil {
		t.Fatal("LastWatchedAt should not be nil")
	}
	want := time.Unix(1800000000, 0).UTC()
	if !show.LastWatchedAt.Equal(want) {
		t.Errorf("LastWatchedAt = %v, want %v (should not be downgraded)", *show.LastWatchedAt, want)
	}
}

func TestHistoryEndpointFailure(t *testing.T) {
	showsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Directory ratingKey="600" type="show" title="TestShow" year="2022" addedAt="1700000000" lastViewedAt="1700100000" leafCount="12"/>
</MediaContainer>`

	ts := newShowHistoryServer(t, showsXML, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal("should not fail when history endpoint fails:", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	want := time.Unix(1700100000, 0).UTC()
	if items[0].LastWatchedAt == nil || !items[0].LastWatchedAt.Equal(want) {
		t.Errorf("LastWatchedAt = %v, want %v (fallback to show-level)", items[0].LastWatchedAt, want)
	}
}

func TestMovieOnlyLibraryNoHistoryCall(t *testing.T) {
	moviesXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Video ratingKey="700" type="movie" title="TestMovie" year="2023" addedAt="1700000000">
    <Media videoResolution="4k"><Part size="30000000000"/></Media>
  </Video>
</MediaContainer>`

	historyCallCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeMovie:
			w.Write([]byte(moviesXML))
		case r.URL.Path == "/library/sections/lib1/all" && r.URL.Query().Get("type") == plexTypeShow:
			w.Write([]byte(`<MediaContainer totalSize="0"/>`))
		case r.URL.Path == "/status/sessions/history/all":
			historyCallCount++
			w.Write([]byte(`<MediaContainer totalSize="0"/>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if historyCallCount != 0 {
		t.Errorf("history endpoint called %d times, want 0 for movie-only library", historyCallCount)
	}
}

func TestHistoryPagination(t *testing.T) {
	showsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer totalSize="1">
  <Directory ratingKey="800" type="show" title="PaginatedShow" year="2023" addedAt="1700000000" leafCount="5"/>
</MediaContainer>`

	callCount := 0
	ts := newShowHistoryServer(t, showsXML, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		start := r.URL.Query().Get("X-Plex-Container-Start")
		total := historyBatchSize + 1
		switch start {
		case "0":
			items := make([]string, historyBatchSize)
			for i := range items {
				items[i] = fmt.Sprintf(`<Video grandparentRatingKey="other%d" viewedAt="%d"/>`, i, 1700500000-i)
			}
			w.Write([]byte(fmt.Sprintf(`<MediaContainer totalSize="%d">%s</MediaContainer>`, total, strings.Join(items, ""))))
		case strconv.Itoa(historyBatchSize):
			w.Write([]byte(fmt.Sprintf(`<MediaContainer totalSize="%d"><Video grandparentRatingKey="800" viewedAt="1700600000"/></MediaContainer>`, total)))
		default:
			w.Write([]byte(`<MediaContainer totalSize="0"/>`))
		}
	})
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetLibraryItems(context.Background(), "lib1")
	if err != nil {
		t.Fatal(err)
	}

	if callCount != 2 {
		t.Errorf("history endpoint called %d times, want 2 (pagination)", callCount)
	}

	show := items[0]
	if show.LastWatchedAt == nil {
		t.Fatal("LastWatchedAt should be set from paginated history")
	}
	want := time.Unix(1700600000, 0).UTC()
	if !show.LastWatchedAt.Equal(want) {
		t.Errorf("LastWatchedAt = %v, want %v", *show.LastWatchedAt, want)
	}
}

func TestEnrichLastWatched_EmptyMap(t *testing.T) {
	items := []models.LibraryItemCache{
		{ItemID: "100", Title: "Test"},
	}
	mediautil.EnrichLastWatched(items, map[string]time.Time{})
	if items[0].LastWatchedAt != nil {
		t.Error("should remain nil with empty history map")
	}
}
