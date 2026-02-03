package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetOIDCSettings_Empty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/oidc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp oidcSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ClientSecret != "" {
		t.Fatalf("expected empty secret, got %q", resp.ClientSecret)
	}
	if resp.Enabled {
		t.Fatal("expected disabled")
	}
}

func TestGetOIDCSettings_MasksSecret(t *testing.T) {
	srv, st := newTestServer(t)

	if err := st.SetSetting("oidc.issuer", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("oidc.client_id", "myid"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("oidc.client_secret", "supersecret"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting("oidc.redirect_url", "https://example.com/callback"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/oidc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp oidcSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ClientSecret != maskedSecret {
		t.Fatalf("expected masked secret %q, got %q", maskedSecret, resp.ClientSecret)
	}
	if resp.Issuer != "https://example.com" {
		t.Fatalf("expected issuer, got %q", resp.Issuer)
	}
}

func TestUpdateOIDCSettings_Saves(t *testing.T) {
	srv, st := newTestServer(t)

	body := `{"issuer":"https://idp.example.com","client_id":"cid","client_secret":"secret123","redirect_url":"https://app/callback"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetOIDCConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != "https://idp.example.com" {
		t.Fatalf("issuer: got %q", cfg.Issuer)
	}
	if cfg.ClientSecret != "secret123" {
		t.Fatalf("secret: got %q", cfg.ClientSecret)
	}
}

func TestUpdateOIDCSettings_MaskedSecretPreservesExisting(t *testing.T) {
	srv, st := newTestServer(t)

	if err := st.SetSetting("oidc.client_secret", "original_secret"); err != nil {
		t.Fatal(err)
	}

	body := `{"issuer":"https://idp.example.com","client_id":"cid","client_secret":"********","redirect_url":"https://app/callback"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := st.GetOIDCConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientSecret != "original_secret" {
		t.Fatalf("expected preserved secret, got %q", cfg.ClientSecret)
	}
}

func TestUpdateOIDCSettings_InvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOIDCSettings_CallsReload(t *testing.T) {
	srv, st := newTestServer(t)

	body := `{"issuer":"","client_id":"","client_secret":"","redirect_url":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetOIDCConfig()
	if cfg.Issuer != "" {
		t.Fatalf("expected empty issuer after clear, got %q", cfg.Issuer)
	}
}

func TestUpdateOIDCSettings_IncompleteConfigRejected(t *testing.T) {
	srv, st := newTestServer(t)

	body := `{"issuer":"https://example.com","client_id":"","client_secret":"","redirect_url":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetOIDCConfig()
	if cfg.Issuer != "" {
		t.Fatalf("expected config not saved, got issuer %q", cfg.Issuer)
	}
}

func TestDeleteOIDCSettings(t *testing.T) {
	srv, st := newTestServer(t)

	st.SetSetting("oidc.issuer", "https://example.com")
	st.SetSetting("oidc.client_id", "cid")
	st.SetSetting("oidc.client_secret", "secret")
	st.SetSetting("oidc.redirect_url", "https://example.com/cb")

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/oidc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetOIDCConfig()
	if cfg.Issuer != "" || cfg.ClientID != "" || cfg.ClientSecret != "" {
		t.Fatalf("expected cleared config, got %+v", cfg)
	}
}

func TestTestOIDCConnection_MissingIssuer(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/oidc/test", strings.NewReader(`{"issuer":""}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestOIDCConnection_InvalidIssuer(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/oidc/test", strings.NewReader(`{"issuer":"http://localhost:99999/nonexistent"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp oidcTestResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure")
	}
	if resp.Error == "" {
		t.Fatal("expected error message")
	}
}
