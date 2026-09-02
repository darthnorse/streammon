package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/store"
)

func getDisplaySettings(t *testing.T, srv *testServer, cookies ...*http.Cookie) displaySettingsResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/display", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp displaySettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func putDisplaySettings(t *testing.T, srv *testServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/settings/display", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestMapSettings(t *testing.T) {
	t.Run("get defaults to the OSM basemap with the dark filter on", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		resp := getDisplaySettings(t, srv)

		if resp.MapTileURL != store.DefaultMapTileURL {
			t.Fatalf("expected %q, got %q", store.DefaultMapTileURL, resp.MapTileURL)
		}
		if resp.MapTileAttribution != "" {
			t.Fatalf("expected empty attribution override, got %q", resp.MapTileAttribution)
		}
		if !resp.MapDarkFilter {
			t.Fatal("expected dark filter enabled by default")
		}
	})

	t.Run("update tile url", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		custom := "https://tiles.example.com/{z}/{x}/{y}.png"
		w := putDisplaySettings(t, srv, `{"map_tile_url":"`+custom+`"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		stored, err := st.GetMapTileURL()
		if err != nil {
			t.Fatal(err)
		}
		if stored != custom {
			t.Fatalf("expected %q stored, got %q", custom, stored)
		}
		if got := getDisplaySettings(t, srv).MapTileURL; got != custom {
			t.Fatalf("expected %q from GET, got %q", custom, got)
		}
	})

	t.Run("update tile url to empty restores the default", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		if err := st.SetMapTileURL("https://tiles.example.com/{z}/{x}/{y}.png"); err != nil {
			t.Fatal(err)
		}

		w := putDisplaySettings(t, srv, `{"map_tile_url":""}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := getDisplaySettings(t, srv).MapTileURL; got != store.DefaultMapTileURL {
			t.Fatalf("expected default, got %q", got)
		}
	})

	t.Run("invalid tile url returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		for _, body := range []string{
			`{"map_tile_url":"http://tiles.example.com/{z}/{x}/{y}.png"}`,
			`{"map_tile_url":"https://tiles.example.com/tiles.png"}`,
			`{"map_tile_url":"javascript:alert(1)"}`,
		} {
			if w := putDisplaySettings(t, srv, body); w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", body, w.Code, w.Body.String())
			}
		}
	})

	t.Run("update attribution", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		w := putDisplaySettings(t, srv, `{"map_tile_attribution":"© CARTO, © OpenStreetMap contributors"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		stored, err := st.GetMapTileAttribution()
		if err != nil {
			t.Fatal(err)
		}
		if stored != "© CARTO, © OpenStreetMap contributors" {
			t.Fatalf("unexpected stored attribution %q", stored)
		}
	})

	t.Run("attribution containing markup returns 400", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		w := putDisplaySettings(t, srv, `{"map_tile_attribution":"<img src=x onerror=alert(1)>"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}

		stored, err := st.GetMapTileAttribution()
		if err != nil {
			t.Fatal(err)
		}
		if stored != "" {
			t.Fatalf("rejected attribution must not be stored, got %q", stored)
		}
	})

	t.Run("update dark filter", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		w := putDisplaySettings(t, srv, `{"map_dark_filter":false}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		on, err := st.GetMapDarkFilter()
		if err != nil {
			t.Fatal(err)
		}
		if on {
			t.Fatal("expected dark filter disabled")
		}
		if getDisplaySettings(t, srv).MapDarkFilter {
			t.Fatal("expected dark filter disabled in GET response")
		}
	})

	t.Run("updating map settings leaves unit system and region alone", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		if err := st.SetUnitSystem("imperial"); err != nil {
			t.Fatal(err)
		}
		if err := st.SetDiscoverRegion("FR"); err != nil {
			t.Fatal(err)
		}

		w := putDisplaySettings(t, srv, `{"map_tile_url":"https://tiles.example.com/{z}/{x}/{y}.png"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		resp := getDisplaySettings(t, srv)
		if resp.UnitSystem != "imperial" {
			t.Fatalf("expected imperial, got %q", resp.UnitSystem)
		}
		if resp.DiscoverRegion != "FR" {
			t.Fatalf("expected FR, got %q", resp.DiscoverRegion)
		}
	})

	t.Run("non-admin can read the tile url but not change it", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		token := createViewerSession(t, st, "viewer")
		cookie := &http.Cookie{Name: auth.CookieName, Value: token}

		if got := getDisplaySettings(t, srv, cookie).MapTileURL; got != store.DefaultMapTileURL {
			t.Fatalf("viewer must be able to read the tile url, got %q", got)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/settings/display",
			strings.NewReader(`{"map_tile_url":"https://evil.example.com/{z}/{x}/{y}.png"}`))
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for a viewer, got %d: %s", w.Code, w.Body.String())
		}
	})
}
