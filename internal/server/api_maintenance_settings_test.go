package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func TestGetMaintenanceSettings_DefaultFalse(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/maintenance", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp maintenanceSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ResolutionWidthAware {
		t.Fatalf("expected resolution_width_aware=false (default), got true")
	}
}

func TestUpdateMaintenanceSettings_RoundTrip(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	body := `{"resolution_width_aware":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/maintenance", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp maintenanceSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if !resp.ResolutionWidthAware {
		t.Fatalf("expected resolution_width_aware=true in PUT response")
	}

	val, err := st.GetMaintenanceResolutionWidthAware()
	if err != nil {
		t.Fatal(err)
	}
	if !val {
		t.Fatalf("expected store to have resolution_width_aware=true")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/settings/maintenance", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 maintenanceSettingsResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if !resp2.ResolutionWidthAware {
		t.Fatalf("expected GET to return resolution_width_aware=true after PUT")
	}
}

func TestUpdateMaintenanceSettings_NonAdminForbidden(t *testing.T) {
	srv, st := newTestServer(t)

	viewerToken := createViewerSession(t, st, "viewer-maintenance")

	body := `{"resolution_width_aware":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/maintenance", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin PUT, got %d: %s", w.Code, w.Body.String())
	}

	_ = models.RoleViewer // ensure import used
}
