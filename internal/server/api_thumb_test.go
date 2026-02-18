package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestThumbProxy_InvalidUserID(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "Emby", Type: models.ServerTypeEmby,
		URL: "http://localhost:8096", APIKey: "k", Enabled: true,
	})

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{"valid guid", "/api/servers/1/thumb/user/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", http.StatusBadGateway}, // valid format, just unreachable
		{"path traversal attempt", "/api/servers/1/thumb/user/../admin", http.StatusBadRequest},
		{"too short", "/api/servers/1/thumb/user/abc", http.StatusBadRequest},
		{"invalid chars", "/api/servers/1/thumb/user/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", http.StatusBadRequest},
		{"with slashes", "/api/servers/1/thumb/user/abc/def/ghi", http.StatusBadRequest},
		{"empty user id", "/api/servers/1/thumb/user/", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("path %q: got status %d, want %d", tt.path, w.Code, tt.wantCode)
			}
		})
	}
}

func TestThumbProxy_MissingPath(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "Plex", Type: models.ServerTypePlex,
		URL: "http://localhost:32400", APIKey: "k", Enabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/servers/1/thumb/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing path, got %d", w.Code)
	}
}

func TestThumbProxy_InvalidServerID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/invalid/thumb/123", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid server id, got %d", w.Code)
	}
}

func TestThumbProxy_ServerNotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/999/thumb/123", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent server, got %d", w.Code)
	}
}

func TestThumbProxy_PathTraversal(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "Plex", Type: models.ServerTypePlex,
		URL: "http://localhost:32400", APIKey: "k", Enabled: true,
	})

	// Note: Query strings (?bar=baz) are parsed by the HTTP router and don't reach
	// the path parameter, so we only test path traversal patterns
	tests := []string{
		"/api/servers/1/thumb/../../../etc/passwd",
		"/api/servers/1/thumb/foo#anchor",
	}

	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("path %q: expected 400 for path traversal, got %d", path, w.Code)
		}
	}
}

func TestThumbProxy_PlexPathRestriction(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	st.CreateServer(&models.Server{
		Name: "Plex", Type: models.ServerTypePlex,
		URL: "http://localhost:32400", APIKey: "k", Enabled: true,
	})

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{"valid thumb path", "/api/servers/1/thumb/library/metadata/12345/thumb", http.StatusBadGateway},
		{"valid thumb with timestamp", "/api/servers/1/thumb/library/metadata/12345/thumb/1700000000", http.StatusBadGateway},
		{"valid actor thumb", "/api/servers/1/thumb/library/metadata/actors/999/thumb", http.StatusBadGateway},
		{"arbitrary api path", "/api/servers/1/thumb/library/sections", http.StatusBadRequest},
		{"status endpoint", "/api/servers/1/thumb/status/sessions", http.StatusBadRequest},
		{"identity endpoint", "/api/servers/1/thumb/identity", http.StatusBadRequest},
		{"accounts endpoint", "/api/servers/1/thumb/accounts", http.StatusBadRequest},
		{"non-numeric id", "/api/servers/1/thumb/library/metadata/abc/thumb", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("path %q: got status %d, want %d", tt.path, w.Code, tt.wantCode)
			}
		})
	}
}

func TestValidPlexThumbPath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"library/metadata/12345/thumb", true},
		{"library/metadata/12345/thumb/1700000000", true},
		{"library/metadata/actors/999/thumb", true},
		{"library/metadata/actors/999/thumb/123", true},
		{"library/sections", false},
		{"status/sessions", false},
		{"identity", false},
		{"library/metadata/abc/thumb", false},
		{"library/metadata/../../../etc/passwd", false},
		{"accounts", false},
	}

	for _, tt := range tests {
		got := validPlexThumbPath.MatchString(tt.path)
		if got != tt.valid {
			t.Errorf("validPlexThumbPath.MatchString(%q) = %v, want %v", tt.path, got, tt.valid)
		}
	}
}

func TestValidUserIDPattern(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", true},  // lowercase hex
		{"A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4", true},  // uppercase hex
		{"0123456789abcdef0123456789abcdef", true},  // mixed
		{"abc", false},                              // too short
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5", false}, // too long
		{"g1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", false}, // invalid char 'g'
		{"a1b2c3d4-e5f6-a1b2-c3d4-e5f6a1b2c3d4", true}, // with dashes (Jellyfin GUID format)
		{"", false},
		{"../../../etc", false},
	}

	for _, tt := range tests {
		got := validUserIDPattern.MatchString(tt.id)
		if got != tt.valid {
			t.Errorf("validUserIDPattern.MatchString(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
}
