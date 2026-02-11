package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/version"
)

func TestHandleVersion(t *testing.T) {
	srv, _ := newTestServer(t)
	vc := version.NewChecker("1.2.3")
	srv.version = vc

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var info version.Info
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Current != "1.2.3" {
		t.Fatalf("expected version=1.2.3, got %s", info.Current)
	}
	if info.UpdateAvailable {
		t.Fatal("expected no update available")
	}
}

func TestHandleVersion_NoChecker(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var info version.Info
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Current != "unknown" {
		t.Fatalf("expected version=unknown, got %s", info.Current)
	}
	if info.UpdateAvailable {
		t.Fatal("expected no update available")
	}
}

func TestHandleVersion_NoAuthRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	vc := version.NewChecker("1.0.0")
	srv.version = vc

	// Request without auth cookie — should still get 200
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", w.Code)
	}
}
