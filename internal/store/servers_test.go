package store

import (
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
