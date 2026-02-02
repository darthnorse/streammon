package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestCreateServerAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"name":"Plex","type":"plex","url":"http://plex:32400","api_key":"abc","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/servers", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var s models.Server
	json.NewDecoder(w.Body).Decode(&s)
	if s.ID == 0 {
		t.Fatal("expected ID")
	}
	if s.Name != "Plex" {
		t.Fatalf("expected Plex, got %s", s.Name)
	}
	if s.APIKey != "" {
		t.Fatal("api_key should not be in JSON response")
	}
}

func TestCreateServerValidationAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"name":"","type":"plex","url":"http://x","api_key":"k"}`},
		{"bad type", `{"name":"X","type":"invalid","url":"http://x","api_key":"k"}`},
		{"empty url", `{"name":"X","type":"plex","url":"","api_key":"k"}`},
		{"empty api_key", `{"name":"X","type":"plex","url":"http://x","api_key":""}`},
		{"invalid json", `{bad`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/servers", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestListServersAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.CreateServer(&models.Server{Name: "A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var servers []models.Server
	json.NewDecoder(w.Body).Decode(&servers)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
}

func TestListServersEmptyAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Body.String() == "null\n" {
		t.Fatal("expected [], got null")
	}
}

func TestGetServerAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.CreateServer(&models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/servers/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/servers/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateServerAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.CreateServer(&models.Server{Name: "Old", Type: models.ServerTypePlex, URL: "http://old", APIKey: "k", Enabled: true})

	body := `{"name":"New","type":"plex","url":"http://new","api_key":"k2","enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/1", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated models.Server
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "New" {
		t.Fatalf("expected New, got %s", updated.Name)
	}
}

func TestUpdateServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"name":"X","type":"plex","url":"http://x","api_key":"k","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/999", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteServerNotFoundAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/servers/999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteServerAPI(t *testing.T) {
	srv, st := newTestServer(t)
	st.CreateServer(&models.Server{Name: "X", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k"})

	req := httptest.NewRequest(http.MethodDelete, "/api/servers/1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}
