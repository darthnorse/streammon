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

func TestGuestSettingsAPI(t *testing.T) {
	t.Run("get returns defaults", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp guestSettingsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !resp.AccessEnabled {
			t.Error("expected access_enabled default true")
		}
		if !resp.VisibleTrustScore {
			t.Error("expected visible_trust_score default true")
		}
		if !resp.ShowDiscover {
			t.Error("expected show_discover default true")
		}
	})

	t.Run("put partial update", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"visible_devices":false,"visible_isps":false}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify via GET
		req2 := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		w2 := httptest.NewRecorder()
		srv.ServeHTTP(w2, req2)

		var resp guestSettingsResponse
		json.NewDecoder(w2.Body).Decode(&resp)

		if resp.VisibleDevices {
			t.Error("expected visible_devices=false")
		}
		if resp.VisibleISPs {
			t.Error("expected visible_isps=false")
		}
		if !resp.VisibleTrustScore {
			t.Error("expected visible_trust_score=true (unchanged)")
		}
	})

	t.Run("viewer can GET but not PUT", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-gs")

		// GET should work for viewers
		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("viewer GET: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// PUT should be 403
		req2 := httptest.NewRequest(http.MethodPut, "/api/settings/guest", strings.NewReader(`{"visible_devices":false}`))
		req2.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w2 := httptest.NewRecorder()
		srv.ServeHTTP(w2, req2)
		if w2.Code != http.StatusForbidden {
			t.Fatalf("viewer PUT: expected 403, got %d: %s", w2.Code, w2.Body.String())
		}
	})

	t.Run("put malformed JSON returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest", strings.NewReader("{bad"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		srv, _ := newTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestGuestVisibilityEnforcement(t *testing.T) {
	t.Run("viewer blocked from trust score when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "alice-vis")
		st.SetGuestSettings(map[string]bool{"visible_trust_score": false})

		req := httptest.NewRequest(http.MethodGet, "/api/users/alice-vis/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer allowed trust score when enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "bob-vis")
		st.SetGuestSettings(map[string]bool{"visible_trust_score": true})

		req := httptest.NewRequest(http.MethodGet, "/api/users/bob-vis/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Fatalf("expected non-403, got 403")
		}
	})

	t.Run("viewer blocked from violations when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "carol-vis")
		st.SetGuestSettings(map[string]bool{"visible_violations": false})

		req := httptest.NewRequest(http.MethodGet, "/api/users/carol-vis/violations?page=1&per_page=10", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer blocked from history when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "dave-vis")
		st.SetGuestSettings(map[string]bool{"visible_watch_history": false})

		req := httptest.NewRequest(http.MethodGet, "/api/history?user=dave-vis&page=1&per_page=10", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer blocked from household when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "eve-vis")
		st.SetGuestSettings(map[string]bool{"visible_household": false})

		req := httptest.NewRequest(http.MethodGet, "/api/users/eve-vis/household", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer stats filters devices and ISPs when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "frank-vis")
		st.SetGuestSettings(map[string]bool{
			"visible_devices": false,
			"visible_isps":    false,
		})

		// Create a user record so stats endpoint does not 404
		st.CreateLocalUser("frank-vis-data", "frank-data@test.local", "", models.RoleViewer)

		req := httptest.NewRequest(http.MethodGet, "/api/users/frank-vis/stats", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var stats models.UserDetailStats
		if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if stats.Devices != nil {
			t.Error("expected devices to be nil when hidden")
		}
		if stats.ISPs != nil {
			t.Error("expected ISPs to be nil when hidden")
		}
	})

	t.Run("admin always sees everything regardless of settings", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		st.SetGuestSettings(map[string]bool{
			"visible_trust_score":   false,
			"visible_violations":    false,
			"visible_watch_history": false,
			"visible_household":     false,
			"visible_devices":       false,
			"visible_isps":          false,
		})

		req := httptest.NewRequest(http.MethodGet, "/api/users/test-admin/trust", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Fatal("admin should not get 403")
		}
	})

	t.Run("viewer blocked from locations when watch history disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "geo-vis")
		st.SetGuestSettings(map[string]bool{"visible_watch_history": false})

		req := httptest.NewRequest(http.MethodGet, "/api/users/geo-vis/locations", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})
}
