package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisplaySettings(t *testing.T) {
	t.Run("get default returns metric", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/display", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp displaySettingsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.UnitSystem != "metric" {
			t.Fatalf("expected metric (default), got %q", resp.UnitSystem)
		}
	})

	t.Run("update to metric", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		body := `{"unit_system":"metric"}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/display", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		val, err := st.GetUnitSystem()
		if err != nil {
			t.Fatal(err)
		}
		if val != "metric" {
			t.Fatalf("expected metric, got %q", val)
		}
	})

	t.Run("update to imperial", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		body := `{"unit_system":"imperial"}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/display", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		val, err := st.GetUnitSystem()
		if err != nil {
			t.Fatal(err)
		}
		if val != "imperial" {
			t.Fatalf("expected imperial, got %q", val)
		}
	})

	t.Run("update with invalid value returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"unit_system":"invalid"}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/display", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("update with invalid JSON returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/display", strings.NewReader("{bad"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("get after update returns updated value", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		if err := st.SetUnitSystem("imperial"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/settings/display", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp displaySettingsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.UnitSystem != "imperial" {
			t.Fatalf("expected imperial, got %q", resp.UnitSystem)
		}
	})
}
