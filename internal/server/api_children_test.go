package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

type mockChildrenServer struct {
	serverID    int64
	details     *models.ItemDetails
	detailsErr  error
	seasons     []models.Season
	episodes    []models.Episode
	seasonsErr  error
	episodesErr error
}

func (m *mockChildrenServer) Name() string                             { return "mock" }
func (m *mockChildrenServer) Type() models.ServerType                  { return models.ServerTypePlex }
func (m *mockChildrenServer) ServerID() int64                          { return m.serverID }
func (m *mockChildrenServer) TestConnection(ctx context.Context) error { return nil }
func (m *mockChildrenServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockChildrenServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockChildrenServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockChildrenServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockChildrenServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockChildrenServer) DeleteItem(ctx context.Context, itemID string) error { return nil }
func (m *mockChildrenServer) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	if m.seasonsErr != nil {
		return nil, m.seasonsErr
	}
	return m.seasons, nil
}
func (m *mockChildrenServer) GetEpisodes(ctx context.Context, seasonID string) ([]models.Episode, error) {
	if m.episodesErr != nil {
		return nil, m.episodesErr
	}
	return m.episodes, nil
}
func (m *mockChildrenServer) TerminateSession(ctx context.Context, sessionID string, message string) error {
	return nil
}
func (m *mockChildrenServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	if m.detailsErr != nil {
		return nil, m.detailsErr
	}
	return m.details, nil
}

func TestChildrenForShowReturnsSeasons(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockChildrenServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID: "show-1", Title: "Show", Level: "show", MediaType: models.MediaTypeTV,
		},
		seasons: []models.Season{
			{ID: "s1", Number: 1, Title: "Season 1", EpisodeCount: 5},
			{ID: "s2", Number: 2, Title: "Season 2", EpisodeCount: 8},
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/children/show-1", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp childrenSeasonsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Seasons) != 2 {
		t.Fatalf("expected 2 seasons, got %d", len(resp.Seasons))
	}
}

func TestChildrenForSeasonReturnsEpisodes(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockChildrenServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID: "season-1", Title: "Season 1", Level: "season", MediaType: models.MediaTypeTV,
		},
		episodes: []models.Episode{
			{ID: "e1", Number: 1, Title: "Pilot"},
			{ID: "e2", Number: 2, Title: "Second"},
			{ID: "e3", Number: 3, Title: "Third"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/children/season-1", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp childrenEpisodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(resp.Episodes))
	}
}

func TestChildrenForMovieReturns400(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockChildrenServer{
		serverID: s.ID,
		details: &models.ItemDetails{
			ID: "movie-1", Title: "Movie", Level: "movie", MediaType: models.MediaTypeMovie,
		},
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/children/movie-1", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChildrenItemNotFound(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	s := &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k", MachineID: "m1", Enabled: true}
	if err := st.CreateServer(s); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	p := setupTestPoller(t, srv.Unwrap(), st)
	p.AddServer(s.ID, &mockChildrenServer{
		serverID:   s.ID,
		detailsErr: models.ErrNotFound,
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/servers/%d/children/missing", s.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChildrenServerNotFound(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	setupTestPoller(t, srv.Unwrap(), st)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/99/children/anything", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
