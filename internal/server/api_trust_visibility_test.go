package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
)

func TestTrustScoreEndpointGuard(t *testing.T) {
	t.Run("viewer cannot access own trust score when setting disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		st.SetGuestSettings(map[string]bool{"visible_trust_score": false})
		viewerToken := createViewerSession(t, st, "viewer-ts1")

		req := httptest.NewRequest(http.MethodGet, "/api/users/viewer-ts1/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer can access own trust score when setting enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		if err := st.SetGuestSettings(map[string]bool{"visible_trust_score": true}); err != nil {
			t.Fatal(err)
		}
		viewerToken := createViewerSession(t, st, "viewer-ts2")

		req := httptest.NewRequest(http.MethodGet, "/api/users/viewer-ts2/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer cannot access other user trust score even when enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		if err := st.SetGuestSettings(map[string]bool{"visible_trust_score": true}); err != nil {
			t.Fatal(err)
		}
		viewerToken := createViewerSession(t, st, "viewer-ts3")

		req := httptest.NewRequest(http.MethodGet, "/api/users/other-user/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin can always access any user trust score", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/users/some-user/trust", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestTrustScoreVisibility(t *testing.T) {
	t.Run("get default returns true", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/trust-visibility", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp trustScoreVisibilityPayload
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled {
			t.Fatal("expected enabled by default")
		}
	})

	t.Run("put enables", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/trust-visibility", strings.NewReader(`{"enabled":true}`))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		enabled, err := st.GetGuestSetting("visible_trust_score")
		if err != nil {
			t.Fatal(err)
		}
		if !enabled {
			t.Fatal("expected enabled after PUT")
		}
	})

	t.Run("put disables", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		if err := st.SetGuestSettings(map[string]bool{"visible_trust_score": true}); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/settings/trust-visibility", strings.NewReader(`{"enabled":false}`))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		enabled, _ := st.GetGuestSetting("visible_trust_score")
		if enabled {
			t.Fatal("expected disabled after PUT false")
		}
	})

	t.Run("viewer can GET", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-tv")

		req := httptest.NewRequest(http.MethodGet, "/api/settings/trust-visibility", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer cannot PUT", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-tv2")

		req := httptest.NewRequest(http.MethodPut, "/api/settings/trust-visibility", strings.NewReader(`{"enabled":true}`))
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})
}
