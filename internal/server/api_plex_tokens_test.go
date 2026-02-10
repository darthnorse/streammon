package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/crypto"
	"streammon/internal/store"
)

func testEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := crypto.NewEncryptor(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func newTestServerWithEncryptor(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	return newTestServer(t, store.WithEncryptor(testEncryptor(t)))
}

func TestPlexTokensSetting_DefaultDisabled(t *testing.T) {
	srv, _ := newTestServerWithEncryptor(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/plex-tokens", nil)
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp plexTokensSettingPayload
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Enabled {
		t.Fatal("expected disabled by default")
	}
	if !resp.Available {
		t.Fatal("expected available when encryptor is set")
	}
}

func TestPlexTokensSetting_NotAvailableWithoutKey(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/plex-tokens", nil)
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp plexTokensSettingPayload
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Available {
		t.Fatal("expected not available without encryptor")
	}
	if resp.Enabled {
		t.Fatal("expected disabled without encryptor")
	}
}

func TestPlexTokensSetting_EnableDisable(t *testing.T) {
	srv, _ := newTestServerWithEncryptor(t)

	// Enable
	req := httptest.NewRequest(http.MethodPut, "/api/settings/plex-tokens", strings.NewReader(`{"enabled":true}`))
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("enable: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify enabled
	req = httptest.NewRequest(http.MethodGet, "/api/settings/plex-tokens", nil)
	addAuthCookie(req)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp plexTokensSettingPayload
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Enabled {
		t.Fatal("expected enabled after PUT")
	}

	// Disable
	req = httptest.NewRequest(http.MethodPut, "/api/settings/plex-tokens", strings.NewReader(`{"enabled":false}`))
	addAuthCookie(req)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("disable: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlexTokensSetting_PutWithoutEncryptor(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/plex-tokens", strings.NewReader(`{"enabled":true}`))
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlexTokensSetting_ViewerCannotAccess(t *testing.T) {
	srv, st := newTestServerWithEncryptor(t)
	viewerToken := createViewerSession(t, st, "viewer-plex")

	req := httptest.NewRequest(http.MethodGet, "/api/settings/plex-tokens", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
