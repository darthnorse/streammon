package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

// seedLibraryItemViaStore creates server id 1 (if not already present) and
// inserts a single library_items row so handler tests have something to query.
func seedLibraryItemViaStore(t *testing.T, st *store.Store, item models.LibraryItemCache) {
	t.Helper()
	ctx := context.Background()

	// Ensure server 1 exists (FK constraint on library_items.server_id).
	_ = st.CreateServer(&models.Server{
		Name: "Test Server", Type: models.ServerTypePlex,
		URL: "http://plex", APIKey: "k", Enabled: true,
	})

	if err := st.SeedLibraryItemsForTest(ctx, []store.LibraryItemSeed{{
		ServerID:  item.ServerID,
		LibraryID: item.LibraryID,
		ItemID:    item.ItemID,
		MediaType: string(item.MediaType),
		Title:     item.Title,
		AddedAt:   item.AddedAt.Format(time.RFC3339),
	}}); err != nil {
		t.Fatalf("seedLibraryItemViaStore: %v", err)
	}
}

func TestListLibraryItemsAPI(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	now := time.Now().UTC()
	seedLibraryItemViaStore(t, st, models.LibraryItemCache{
		ServerID: 1, LibraryID: "1",
		ItemID: "m1", MediaType: models.MediaTypeMovie, Title: "Dune", AddedAt: now, FileSize: 5,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/1/items?page=1&per_page=20", nil)
	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got models.PaginatedResult[models.LibraryItemDetail]
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Title != "Dune" {
		t.Errorf("got %+v, want one Dune item", got)
	}
}

func TestLibrarySummaryAPI(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	seedLibraryItemViaStore(t, st, models.LibraryItemCache{
		ServerID: 1, LibraryID: "1",
		ItemID: "m1", MediaType: models.MediaTypeMovie, Title: "Dune",
		AddedAt: time.Now().UTC(), FileSize: 5,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/1/summary", nil)
	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var got models.LibrarySummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalTitles != 1 || got.NeverPlayed != 1 {
		t.Errorf("summary=%+v", got)
	}
}

func TestListLibraryItemsCSV(t *testing.T) {
	ts, st := newTestServerWrapped(t)
	seedLibraryItemViaStore(t, st, models.LibraryItemCache{
		ServerID: 1, LibraryID: "1",
		ItemID: "m1", MediaType: models.MediaTypeMovie, Title: "Dune",
		AddedAt: time.Now().UTC(), FileSize: 5,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/1/1/items?format=csv", nil)
	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("content-type=%q want text/csv", ct)
	}
}
