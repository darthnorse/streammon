package mediautil

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"streammon/internal/models"
)

// TestEnrichSeriesDataConcurrent drives many series through the parallel size
// fetch; run with -race it guards against a data race on the shared slice.
func TestEnrichSeriesDataConcurrent(t *testing.T) {
	const n = 60
	series := make([]models.LibraryItemCache, n)
	for i := range series {
		series[i] = models.LibraryItemCache{ItemID: strconv.Itoa(i)}
	}

	var calls int64
	fetchSize := func(_ context.Context, itemID string) (int64, error) {
		atomic.AddInt64(&calls, 1)
		id, _ := strconv.Atoi(itemID)
		return int64(id + 1), nil // distinct non-zero size per series
	}

	EnrichSeriesData(context.Background(), series, "lib1", "plex", fetchSize)

	if calls != n {
		t.Errorf("fetchSize called %d times, want %d", calls, n)
	}
	for i := range series {
		if want := int64(i + 1); series[i].FileSize != want {
			t.Errorf("series[%d].FileSize = %d, want %d", i, series[i].FileSize, want)
		}
	}
}

func TestEnrichSeriesDataSkipsNonZero(t *testing.T) {
	series := []models.LibraryItemCache{{ItemID: "a", FileSize: 100}}
	called := false
	EnrichSeriesData(context.Background(), series, "lib1", "plex",
		func(context.Context, string) (int64, error) { called = true; return 999, nil })

	if called {
		t.Error("fetchSize should not be called for items that already have a FileSize")
	}
	if series[0].FileSize != 100 {
		t.Errorf("FileSize changed to %d, want 100 (unchanged)", series[0].FileSize)
	}
}
