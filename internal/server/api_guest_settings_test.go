package server

import (
	"encoding/json"
	"fmt"
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

		if resp.Settings["access_enabled"] {
			t.Error("expected access_enabled default false")
		}
		if !resp.Settings["visible_trust_score"] {
			t.Error("expected visible_trust_score default true")
		}
		if !resp.Settings["show_discover"] {
			t.Error("expected show_discover default true")
		}
		if !resp.Settings["show_calendar"] {
			t.Error("expected show_calendar default true")
		}
		if resp.Settings["store_plex_tokens"] {
			t.Error("expected store_plex_tokens default false")
		}
		if !resp.Settings["visible_profile"] {
			t.Error("expected visible_profile default true")
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

		req2 := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		w2 := httptest.NewRecorder()
		srv.ServeHTTP(w2, req2)

		var resp guestSettingsResponse
		if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.Settings["visible_devices"] {
			t.Error("expected visible_devices=false")
		}
		if resp.Settings["visible_isps"] {
			t.Error("expected visible_isps=false")
		}
		if !resp.Settings["visible_trust_score"] {
			t.Error("expected visible_trust_score=true (unchanged)")
		}
	})

	t.Run("viewer can GET but not PUT", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-gs")

		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("viewer GET: expected 200, got %d: %s", w.Code, w.Body.String())
		}

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

	t.Run("put empty body returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest", strings.NewReader("{}"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("put unknown key returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest", strings.NewReader(`{"bogus_key":true}`))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
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
	blockedCases := []struct {
		name        string
		viewerName  string
		setting     string
		urlSuffix   string
		urlOverride string
	}{
		{"trust score", "alice-vis", "visible_trust_score", "/trust", ""},
		{"violations", "carol-vis", "visible_violations", "/violations?page=1&per_page=10", ""},
		{"history", "dave-vis", "visible_watch_history", "", "/api/history?user=%s&page=1&per_page=10"},
		{"daily history", "daily-vis", "visible_watch_history", "", "/api/history/daily?start=2025-01-01&end=2025-01-31"},
		{"sessions", "sess-vis", "visible_watch_history", "", "/api/history/1/sessions"},
		{"household", "eve-vis", "visible_household", "/household", ""},
		{"locations", "geo-vis", "visible_watch_history", "/locations", ""},
	}

	for _, tc := range blockedCases {
		t.Run("viewer blocked from "+tc.name+" when disabled", func(t *testing.T) {
			srv, st := newTestServer(t)
			viewerName := tc.viewerName
			viewerToken := createViewerSession(t, st, viewerName)
			if err := st.SetGuestSettings(map[string]bool{tc.setting: false}); err != nil {
				t.Fatalf("SetGuestSettings: %v", err)
			}

			url := "/api/users/" + viewerName + tc.urlSuffix
			if tc.urlOverride != "" {
				if strings.Contains(tc.urlOverride, "%s") {
					url = fmt.Sprintf(tc.urlOverride, viewerName)
				} else {
					url = tc.urlOverride
				}
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
			}
		})
	}

	t.Run("visible_profile=false blocks all profile endpoints", func(t *testing.T) {
		profileEndpoints := []struct {
			name        string
			viewerName  string
			urlSuffix   string
			urlOverride string
		}{
			{"trust score", "prof-trust", "/trust", ""},
			{"violations", "prof-viol", "/violations?page=1&per_page=10", ""},
			{"stats", "prof-stats", "/stats", ""},
			{"household", "prof-hh", "/household", ""},
			{"locations", "prof-loc", "/locations", ""},
			{"history", "prof-hist", "", "/api/history?user=%s&page=1&per_page=10"},
			{"daily history", "prof-daily", "", "/api/history/daily?start=2025-01-01&end=2025-01-31"},
			{"sessions", "prof-sess", "", "/api/history/1/sessions"},
		}
		for _, ep := range profileEndpoints {
			t.Run(ep.name, func(t *testing.T) {
				srv, st := newTestServer(t)
				viewerToken := createViewerSession(t, st, ep.viewerName)
				if err := st.SetGuestSettings(map[string]bool{"visible_profile": false}); err != nil {
					t.Fatalf("SetGuestSettings: %v", err)
				}

				url := "/api/users/" + ep.viewerName + ep.urlSuffix
				if ep.urlOverride != "" {
					if strings.Contains(ep.urlOverride, "%s") {
						url = fmt.Sprintf(ep.urlOverride, ep.viewerName)
					} else {
						url = ep.urlOverride
					}
				}
				req := httptest.NewRequest(http.MethodGet, url, nil)
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
				w := httptest.NewRecorder()
				srv.ServeHTTP(w, req)

				if w.Code != http.StatusForbidden {
					t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
				}
			})
		}
	})

	t.Run("admin bypasses visible_profile=false", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		if err := st.SetGuestSettings(map[string]bool{"visible_profile": false}); err != nil {
			t.Fatalf("SetGuestSettings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/users/test-admin/trust", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Fatal("admin should not get 403 when visible_profile is false")
		}
	})

	t.Run("viewer allowed trust score when enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "bob-vis")
		if err := st.SetGuestSettings(map[string]bool{"visible_trust_score": true}); err != nil {
			t.Fatalf("SetGuestSettings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/users/bob-vis/trust", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Fatalf("expected non-403, got 403")
		}
	})

	t.Run("viewer stats filters devices and ISPs when disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "frank-vis")
		if err := st.SetGuestSettings(map[string]bool{
			"visible_devices": false,
			"visible_isps":    false,
		}); err != nil {
			t.Fatalf("SetGuestSettings: %v", err)
		}

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
		if err := st.SetGuestSettings(map[string]bool{
			"visible_profile":       false,
			"visible_trust_score":   false,
			"visible_violations":    false,
			"visible_watch_history": false,
			"visible_household":     false,
			"visible_devices":       false,
			"visible_isps":          false,
		}); err != nil {
			t.Fatalf("SetGuestSettings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/users/test-admin/trust", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Fatal("admin should not get 403")
		}
	})

	t.Run("viewer blocked from other user household", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "hh-viewer")

		req := httptest.NewRequest(http.MethodGet, "/api/users/test-admin/household", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	calendarBlockedEndpoints := []struct {
		name string
		url  string
	}{
		{"calendar", "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07"},
		{"series", "/api/sonarr/series/10"},
		{"poster", "/api/sonarr/poster/10"},
	}
	for _, ep := range calendarBlockedEndpoints {
		t.Run("viewer blocked from "+ep.name+" when show_calendar disabled", func(t *testing.T) {
			mock := mockSonarr(t)
			srv, st := newTestServer(t)
			configureSonarr(t, st, mock.URL)
			viewerToken := createViewerSession(t, st, ep.name+"-viewer")
			if err := st.SetGuestSettings(map[string]bool{"show_calendar": false}); err != nil {
				t.Fatalf("SetGuestSettings: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, ep.url, nil)
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
			}
		})
	}

	t.Run("viewer allowed calendar when show_calendar enabled", func(t *testing.T) {
		mock := mockSonarr(t)
		srv, st := newTestServer(t)
		configureSonarr(t, st, mock.URL)
		viewerToken := createViewerSession(t, st, "cal-allowed")

		req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin bypasses show_calendar=false", func(t *testing.T) {
		srv, st := newSonarrTestServer(t)
		if err := st.SetGuestSettings(map[string]bool{"show_calendar": false}); err != nil {
			t.Fatalf("SetGuestSettings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/sonarr/calendar?start=2025-03-01&end=2025-03-07", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}
