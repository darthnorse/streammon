package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdleTimeoutSettings(t *testing.T) {
	t.Run("get default returns 5", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/idle-timeout", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp idleTimeoutPayload
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.IdleTimeoutMinutes != 5 {
			t.Fatalf("expected 5 (default), got %d", resp.IdleTimeoutMinutes)
		}
	})

	t.Run("put valid value returns 200", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		body := `{"idle_timeout_minutes":10}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/idle-timeout", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		val, err := st.GetIdleTimeoutMinutes()
		if err != nil {
			t.Fatal(err)
		}
		if val != 10 {
			t.Fatalf("expected 10, got %d", val)
		}
	})

	t.Run("put zero disables", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		body := `{"idle_timeout_minutes":0}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/idle-timeout", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		val, _ := st.GetIdleTimeoutMinutes()
		if val != 0 {
			t.Fatalf("expected 0 (disabled), got %d", val)
		}
	})

	t.Run("put negative returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"idle_timeout_minutes":-1}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/idle-timeout", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("put exceeds max returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		body := `{"idle_timeout_minutes":1441}`
		req := httptest.NewRequest(http.MethodPut, "/api/settings/idle-timeout", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("put malformed JSON returns 400", func(t *testing.T) {
		srv, _ := newTestServerWrapped(t)

		req := httptest.NewRequest(http.MethodPut, "/api/settings/idle-timeout", strings.NewReader("{bad"))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("get after update returns updated value", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)

		if err := st.SetIdleTimeoutMinutes(15); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/settings/idle-timeout", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp idleTimeoutPayload
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.IdleTimeoutMinutes != 15 {
			t.Fatalf("expected 15, got %d", resp.IdleTimeoutMinutes)
		}
	})
}
