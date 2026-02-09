package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuestAccess(t *testing.T) {
	t.Run("get default returns false", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest-access", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp guestAccessPayload
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Enabled {
			t.Fatal("expected disabled by default")
		}
	})

	t.Run("put enables guest access", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest-access", strings.NewReader(`{"enabled":true}`))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		enabled, err := st.GetGuestAccess()
		if err != nil {
			t.Fatal(err)
		}
		if !enabled {
			t.Fatal("expected enabled after PUT")
		}
	})

	t.Run("put disables guest access", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		if err := st.SetGuestAccess(true); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest-access", strings.NewReader(`{"enabled":false}`))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		enabled, _ := st.GetGuestAccess()
		if enabled {
			t.Fatal("expected disabled after PUT false")
		}
	})

	t.Run("get after enable returns true", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		if err := st.SetGuestAccess(true); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/settings/guest-access", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp guestAccessPayload
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled {
			t.Fatal("expected enabled")
		}
	})

	t.Run("put malformed JSON returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/guest-access", strings.NewReader("{bad"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}
