package store

import (
	"testing"
	"time"

	"streammon/internal/models"
)

func TestSetAndGetCachedGeo(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	geo := &models.GeoResult{
		IP: "8.8.8.8", Lat: 37.386, Lng: -122.084, City: "Mountain View", Country: "US",
	}
	if err := s.SetCachedGeo(geo); err != nil {
		t.Fatalf("SetCachedGeo: %v", err)
	}

	got, err := s.GetCachedGeo("8.8.8.8")
	if err != nil {
		t.Fatalf("GetCachedGeo: %v", err)
	}
	if got == nil {
		t.Fatal("expected cached result, got nil")
	}
	if got.City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", got.City)
	}
	if got.Country != "US" {
		t.Fatalf("expected US, got %s", got.Country)
	}
}

func TestGetCachedGeoMiss(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	got, err := s.GetCachedGeo("1.2.3.4")
	if err != nil {
		t.Fatalf("GetCachedGeo: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestGetCachedGeoExpired(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	geo := &models.GeoResult{
		IP: "8.8.8.8", Lat: 37.386, Lng: -122.084, City: "Mountain View", Country: "US",
	}
	if err := s.SetCachedGeo(geo); err != nil {
		t.Fatal(err)
	}
	_, err := s.db.Exec("UPDATE ip_geo_cache SET cached_at = ? WHERE ip = ?",
		time.Now().UTC().Add(-31*24*time.Hour), "8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCachedGeo("8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil for expired entry")
	}
}

func TestSetCachedGeoUpsert(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetCachedGeo(&models.GeoResult{IP: "8.8.8.8", City: "Old"}); err != nil {
		t.Fatalf("SetCachedGeo: %v", err)
	}
	if err := s.SetCachedGeo(&models.GeoResult{IP: "8.8.8.8", City: "New"}); err != nil {
		t.Fatalf("SetCachedGeo: %v", err)
	}

	got, err := s.GetCachedGeo("8.8.8.8")
	if err != nil {
		t.Fatalf("GetCachedGeo: %v", err)
	}
	if got.City != "New" {
		t.Fatalf("expected New, got %s", got.City)
	}
}

func TestGetCachedGeoBatch(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetCachedGeo(&models.GeoResult{IP: "8.8.8.8", City: "Mountain View", Country: "US"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCachedGeo(&models.GeoResult{IP: "1.1.1.1", City: "Sydney", Country: "AU"}); err != nil {
		t.Fatal(err)
	}

	result, err := s.GetCachedGeos([]string{"8.8.8.8", "1.1.1.1", "9.9.9.9"})
	if err != nil {
		t.Fatalf("GetCachedGeos: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result["8.8.8.8"].City != "Mountain View" {
		t.Fatalf("expected Mountain View, got %s", result["8.8.8.8"].City)
	}
	if result["1.1.1.1"].City != "Sydney" {
		t.Fatalf("expected Sydney, got %s", result["1.1.1.1"].City)
	}
}

func TestGetCachedGeoBatchEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	result, err := s.GetCachedGeos([]string{})
	if err != nil {
		t.Fatalf("GetCachedGeos: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}

func TestDistinctIPsForUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	earlier := now.Add(-time.Hour)
	entries := []models.WatchHistoryEntry{
		{ServerID: srv.ID, UserName: "alice", MediaType: "movie", Title: "A", IPAddress: "8.8.8.8", StartedAt: earlier, StoppedAt: earlier},
		{ServerID: srv.ID, UserName: "alice", MediaType: "movie", Title: "B", IPAddress: "8.8.8.8", StartedAt: now, StoppedAt: now},
		{ServerID: srv.ID, UserName: "alice", MediaType: "movie", Title: "C", IPAddress: "1.1.1.1", StartedAt: earlier, StoppedAt: earlier},
		{ServerID: srv.ID, UserName: "bob", MediaType: "movie", Title: "D", IPAddress: "2.2.2.2", StartedAt: now, StoppedAt: now},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	results, err := s.DistinctIPsForUser("alice")
	if err != nil {
		t.Fatalf("DistinctIPsForUser: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 distinct IPs, got %d", len(results))
	}
	// Results should be ordered by last_seen DESC, so 8.8.8.8 (more recent) comes first
	if results[0].IP != "8.8.8.8" {
		t.Errorf("expected first IP to be 8.8.8.8, got %s", results[0].IP)
	}
	if results[0].LastSeen.Before(results[1].LastSeen) {
		t.Errorf("expected results ordered by last_seen DESC")
	}
}
