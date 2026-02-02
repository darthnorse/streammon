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
		time.Now().Add(-31*24*time.Hour), "8.8.8.8")
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

	geo1 := &models.GeoResult{IP: "8.8.8.8", City: "Old"}
	s.SetCachedGeo(geo1)

	geo2 := &models.GeoResult{IP: "8.8.8.8", City: "New"}
	s.SetCachedGeo(geo2)

	got, _ := s.GetCachedGeo("8.8.8.8")
	if got.City != "New" {
		t.Fatalf("expected New, got %s", got.City)
	}
}

func TestGetCachedGeoBatch(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetCachedGeo(&models.GeoResult{IP: "8.8.8.8", City: "Mountain View", Country: "US"})
	s.SetCachedGeo(&models.GeoResult{IP: "1.1.1.1", City: "Sydney", Country: "AU"})

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

	s.db.Exec(`INSERT INTO servers (name, type, url, api_key) VALUES ('S','plex','http://s','k')`)

	now := time.Now()
	s.db.Exec(`INSERT INTO watch_history (server_id, user_name, media_type, title, ip_address, started_at, stopped_at)
		VALUES (1, 'alice', 'movie', 'A', '8.8.8.8', ?, ?)`, now, now)
	s.db.Exec(`INSERT INTO watch_history (server_id, user_name, media_type, title, ip_address, started_at, stopped_at)
		VALUES (1, 'alice', 'movie', 'B', '8.8.8.8', ?, ?)`, now, now)
	s.db.Exec(`INSERT INTO watch_history (server_id, user_name, media_type, title, ip_address, started_at, stopped_at)
		VALUES (1, 'alice', 'movie', 'C', '1.1.1.1', ?, ?)`, now, now)
	s.db.Exec(`INSERT INTO watch_history (server_id, user_name, media_type, title, ip_address, started_at, stopped_at)
		VALUES (1, 'bob', 'movie', 'D', '2.2.2.2', ?, ?)`, now, now)

	ips, err := s.DistinctIPsForUser("alice")
	if err != nil {
		t.Fatalf("DistinctIPsForUser: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 distinct IPs, got %d", len(ips))
	}
}
