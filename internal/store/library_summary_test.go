package store

import (
	"context"
	"testing"
)

func TestLibrarySummary_AggregatesByServerAndMediaType(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed servers to satisfy FK constraint.
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO servers(id, name, type, url, api_key) VALUES
		 (1,'Server1','plex','http://s1','key1'),
		 (2,'Server2','plex','http://s2','key2')`,
	); err != nil {
		t.Fatalf("seed servers: %v", err)
	}

	// Seed: server 1 has 2 movies + 1 series with 3 episodes; server 2 has 1 movie.
	// Server 1 asserts: TotalItems=3 (2 movies + 1 series row), Movies=2, Shows=1, Episodes=3, Libraries=2.
	// Server 2 asserts: TotalItems=1, Movies=1, Shows=0, Episodes=0, Libraries=1.
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO library_items
		   (server_id, library_id, item_id, media_type, title, year, added_at, episode_count)
		 VALUES
		   (1, 'lib1', 'i1', 'movie',   'A',     2020, '2024-01-01', 0),
		   (1, 'lib1', 'i2', 'movie',   'B',     2021, '2024-01-02', 0),
		   (1, 'lib2', 'i3', 'episode', 'Show C', 2019, '2024-01-03', 3),
		   (2, 'lib3', 'i4', 'movie',   'F',     2022, '2024-01-04', 0)`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := s.LibrarySummary(ctx)
	if err != nil {
		t.Fatalf("LibrarySummary: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 server entries, got %d", len(got))
	}

	byServer := map[int64]LibraryServerSummary{}
	for _, e := range got {
		byServer[e.ServerID] = e
	}

	s1 := byServer[1]
	if s1.Movies != 2 || s1.Shows != 1 || s1.Episodes != 3 || s1.Libraries != 2 || s1.TotalItems != 3 {
		t.Errorf("server 1 wrong: %+v", s1)
	}
	s2 := byServer[2]
	if s2.Movies != 1 || s2.Shows != 0 || s2.Episodes != 0 || s2.Libraries != 1 || s2.TotalItems != 1 {
		t.Errorf("server 2 wrong: %+v", s2)
	}
}

func TestLibrarySummary_EmptyTable(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	got, err := s.LibrarySummary(context.Background())
	if err != nil {
		t.Fatalf("LibrarySummary: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}
