package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/auth"
)

func TestGetOIDCSettings_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

	for _, kv := range [][2]string{
		{"oidc.issuer", "https://example.com"},
		{"oidc.client_id", "myid"},
		{"oidc.client_secret", "supersecret"},
		{"oidc.redirect_url", "https://example.com/callback"},
	} {
		if err := st.SetSetting(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
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
	srv, st := newTestServerWrapped(t)

	body := `{"issuer":"https://idp.example.com","client_id":"cid","client_secret":"secret123","redirect_url":"https://app/callback","admin_group":"streammon-admins","scopes":"openid,profile,email"}`
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
	if cfg.AdminGroup != "streammon-admins" {
		t.Fatalf("admin_group: got %q", cfg.AdminGroup)
	}
	if cfg.Scopes != "openid,profile,email" {
		t.Fatalf("scopes: got %q", cfg.Scopes)
	}
}

func TestUpdateOIDCSettings_FieldsTrimmed(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	body := `{"issuer":"  https://idp.example.com  ","client_id":"  cid  ","client_secret":"secret123","redirect_url":"  https://app/callback  ","admin_group":"  my-admins  ","scopes":"  openid,profile  "}`
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
		t.Fatalf("expected trimmed issuer, got %q", cfg.Issuer)
	}
	if cfg.ClientID != "cid" {
		t.Fatalf("expected trimmed client_id, got %q", cfg.ClientID)
	}
	if cfg.RedirectURL != "https://app/callback" {
		t.Fatalf("expected trimmed redirect_url, got %q", cfg.RedirectURL)
	}
	if cfg.AdminGroup != "my-admins" {
		t.Fatalf("expected trimmed admin_group, got %q", cfg.AdminGroup)
	}
	if cfg.Scopes != "openid,profile" {
		t.Fatalf("expected trimmed scopes, got %q", cfg.Scopes)
	}
}

func TestGetOIDCSettings_ReturnsAdminGroup(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	for _, kv := range [][2]string{
		{"oidc.issuer", "https://example.com"},
		{"oidc.client_id", "cid"},
		{"oidc.client_secret", "secret"},
		{"oidc.redirect_url", "https://example.com/cb"},
		{"oidc.admin_group", "my-admins"},
	} {
		if err := st.SetSetting(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
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
	if resp.AdminGroup != "my-admins" {
		t.Fatalf("expected admin_group %q, got %q", "my-admins", resp.AdminGroup)
	}
	if resp.Scopes != auth.DefaultScopes {
		t.Fatalf("expected default scopes %q, got %q", auth.DefaultScopes, resp.Scopes)
	}
}

func TestUpdateOIDCSettings_MaskedSecretPreservesExisting(t *testing.T) {
	srv, st := newTestServerWrapped(t)

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
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/oidc", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOIDCSettings_CallsReload(t *testing.T) {
	srv, st := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

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
	srv, st := newTestServerWrapped(t)

	for _, kv := range [][2]string{
		{"oidc.issuer", "https://example.com"},
		{"oidc.client_id", "cid"},
		{"oidc.client_secret", "secret"},
		{"oidc.redirect_url", "https://example.com/cb"},
		{"oidc.admin_group", "my-admins"},
		{"oidc.scopes", "openid,profile"},
	} {
		if err := st.SetSetting(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/oidc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, _ := st.GetOIDCConfig()
	if cfg.Issuer != "" || cfg.ClientID != "" || cfg.ClientSecret != "" || cfg.AdminGroup != "" || cfg.Scopes != "" {
		t.Fatalf("expected cleared config, got %+v", cfg)
	}
}

func TestTestOIDCConnection_MissingIssuer(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/oidc/test", strings.NewReader(`{"issuer":""}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestOIDCConnection_InvalidIssuer(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/oidc/test", strings.NewReader(`{"issuer":"https://localhost:99999/nonexistent"}`))
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
