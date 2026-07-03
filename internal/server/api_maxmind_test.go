package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetMaxMindSettings_Empty(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/maxmind", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp maxmindSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LicenseKey != "" {
		t.Fatalf("expected empty license key, got %q", resp.LicenseKey)
	}
}

func TestUpdateMaxMindSettings_EncryptsAtRest(t *testing.T) {
	srv, st := newTestServerWithEncryptor(t)
	wrapped := &testServer{srv}

	body := `{"license_key":"supersecretmmkey"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/maxmind", strings.NewReader(body))
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	raw, err := st.GetSetting("maxmind.license_key")
	if err != nil {
		t.Fatal(err)
	}
	if raw == "supersecretmmkey" {
		t.Fatal("license key stored in plaintext despite encryptor being configured")
	}
	if !strings.HasPrefix(raw, "enc:") {
		t.Fatalf("expected enc: prefix, got %q", raw)
	}

	got, err := st.GetMaxMindLicenseKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != "supersecretmmkey" {
		t.Fatalf("expected decrypted key on read-back, got %q", got)
	}
}

func TestGetMaxMindSettings_MasksSecret(t *testing.T) {
	srv, st := newTestServerWithEncryptor(t)
	wrapped := &testServer{srv}

	if err := st.SetMaxMindLicenseKey("supersecretmmkey"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/maxmind", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp maxmindSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(resp.LicenseKey, "supersecretmmkey") {
		t.Fatalf("full license key leaked in response: %q", resp.LicenseKey)
	}
	if resp.LicenseKey != "****mkey" {
		t.Fatalf("expected partial mask, got %q", resp.LicenseKey)
	}
}

func TestGetMaxMindSettings_UndecryptableShowsMaskedSecret(t *testing.T) {
	// Value encrypted by one store, read back by a server with no encryptor configured.
	encSrv, encStore := newTestServerWithEncryptor(t)
	_ = encSrv
	if err := encStore.SetMaxMindLicenseKey("supersecretmmkey"); err != nil {
		t.Fatal(err)
	}
	raw, err := encStore.GetSetting("maxmind.license_key")
	if err != nil {
		t.Fatal(err)
	}

	srv, st := newTestServer(t) // no encryptor
	wrapped := &testServer{srv}
	if err := st.SetSetting("maxmind.license_key", raw); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/maxmind", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp maxmindSettingsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LicenseKey != maskedSecret {
		t.Fatalf("expected masked secret placeholder %q, got %q", maskedSecret, resp.LicenseKey)
	}
}

func TestDeleteMaxMindSettings_ClearsKey(t *testing.T) {
	srv, st := newTestServerWithEncryptor(t)
	wrapped := &testServer{srv}

	if err := st.SetMaxMindLicenseKey("supersecretmmkey"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/maxmind", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := st.GetMaxMindLicenseKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("expected cleared key, got %q", got)
	}
}
