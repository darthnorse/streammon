package store

import (
	"errors"
	"testing"

	"streammon/internal/models"
)

func TestCreateServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{
		Name:    "My Plex",
		Type:    models.ServerTypePlex,
		URL:     "http://localhost:32400",
		APIKey:  "abc123",
		Enabled: true,
	}
	err := s.CreateServer(srv)
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if srv.ID == 0 {
		t.Fatal("expected ID to be set")
	}
}

func TestGetServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	got, err := s.GetServer(srv.ID)
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if got.Name != "Plex" {
		t.Fatalf("expected name Plex, got %s", got.Name)
	}
	if !got.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestGetServerNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	_, err := s.GetServer(999)
	if err == nil {
		t.Fatal("expected error for non-existent server")
	}
}

func TestListServers(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.CreateServer(&models.Server{Name: "A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "k"})
	s.CreateServer(&models.Server{Name: "B", Type: models.ServerTypeEmby, URL: "http://b", APIKey: "k"})

	servers, err := s.ListServers()
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestUpdateServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Old", Type: models.ServerTypePlex, URL: "http://old", APIKey: "k", Enabled: true}
	s.CreateServer(srv)

	srv.Name = "New"
	srv.Enabled = false
	err := s.UpdateServer(srv)
	if err != nil {
		t.Fatalf("UpdateServer: %v", err)
	}

	got, _ := s.GetServer(srv.ID)
	if got.Name != "New" {
		t.Fatalf("expected New, got %s", got.Name)
	}
	if got.Enabled {
		t.Fatal("expected disabled")
	}
}

func TestDeleteServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "X", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k"}
	s.CreateServer(srv)

	err := s.DeleteServer(srv.ID)
	if err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}

	_, err = s.GetServer(srv.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestSoftDeleteServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	if err := s.SoftDeleteServer(srv.ID); err != nil {
		t.Fatalf("SoftDeleteServer: %v", err)
	}

	// ListServers should exclude soft-deleted
	servers, err := s.ListServers()
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0 active servers, got %d", len(servers))
	}

	// ListAllServers should include it with DeletedAt set
	all, err := s.ListAllServers()
	if err != nil {
		t.Fatalf("ListAllServers: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 server in all, got %d", len(all))
	}
	if all[0].DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
}

func TestSoftDeleteServerPreservesHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	_, err := s.db.Exec(
		`INSERT INTO watch_history (server_id, user_name, title, media_type, started_at, stopped_at)
		VALUES (?, 'user1', 'Movie', 'movie', '2024-01-01T00:00:00Z', '2024-01-01T02:00:00Z')`,
		srv.ID,
	)
	if err != nil {
		t.Fatalf("inserting history: %v", err)
	}

	if err := s.SoftDeleteServer(srv.ID); err != nil {
		t.Fatalf("SoftDeleteServer: %v", err)
	}

	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE server_id = ?`, srv.ID).Scan(&count)
	if err != nil {
		t.Fatalf("counting history: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 history row, got %d", count)
	}
}

func TestHardDeleteServerRemovesHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	_, err := s.db.Exec(
		`INSERT INTO watch_history (server_id, user_name, title, media_type, started_at, stopped_at)
		VALUES (?, 'user1', 'Movie', 'movie', '2024-01-01T00:00:00Z', '2024-01-01T02:00:00Z')`,
		srv.ID,
	)
	if err != nil {
		t.Fatalf("inserting history: %v", err)
	}

	if err := s.DeleteServer(srv.ID); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}

	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE server_id = ?`, srv.ID).Scan(&count)
	if err != nil {
		t.Fatalf("counting history: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 history rows, got %d", count)
	}
}

func TestRestoreServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	if err := s.SoftDeleteServer(srv.ID); err != nil {
		t.Fatalf("SoftDeleteServer: %v", err)
	}

	if err := s.RestoreServer(srv.ID); err != nil {
		t.Fatalf("RestoreServer: %v", err)
	}

	// Should be back in ListServers
	servers, err := s.ListServers()
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].DeletedAt != nil {
		t.Fatal("expected DeletedAt to be nil after restore")
	}
}

func TestSoftDeleteServerNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	err := s.SoftDeleteServer(999)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRestoreActiveServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	err := s.RestoreServer(srv.ID)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for restoring active server, got %v", err)
	}
}

func TestDoubleSoftDelete(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex:32400", APIKey: "key", Enabled: true}
	s.CreateServer(srv)

	if err := s.SoftDeleteServer(srv.ID); err != nil {
		t.Fatalf("first SoftDeleteServer: %v", err)
	}

	err := s.SoftDeleteServer(srv.ID)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on double soft-delete, got %v", err)
	}
}
