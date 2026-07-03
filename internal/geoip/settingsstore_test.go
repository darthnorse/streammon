package geoip

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"streammon/internal/store"
)

// fakeSettingsStore is a minimal in-memory SettingsStore for testing Updater
// behavior around the (encrypted) MaxMind license key.
type fakeSettingsStore struct {
	licenseKey string
	settings   map[string]string
}

func (f *fakeSettingsStore) GetSetting(key string) (string, error) {
	return f.settings[key], nil
}

func (f *fakeSettingsStore) SetSetting(key, value string) error {
	if f.settings == nil {
		f.settings = map[string]string{}
	}
	f.settings[key] = value
	return nil
}

func (f *fakeSettingsStore) GetMaxMindLicenseKey() (string, error) {
	return f.licenseKey, nil
}

// TestDownloadSkipsWithoutUsableKey verifies download() doesn't hit the
// network (and doesn't error) when no license key is configured, or when
// the stored key is encrypted but undecryptable (EncryptedPlaceholder) —
// e.g. a server started without TOKEN_ENCRYPTION_KEY.
func TestDownloadSkipsWithoutUsableKey(t *testing.T) {
	for name, key := range map[string]string{
		"empty":       "",
		"placeholder": store.EncryptedPlaceholder,
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			}))
			defer ts.Close()

			fs := &fakeSettingsStore{licenseKey: key}
			u := &Updater{
				store:           fs,
				resolver:        &Resolver{},
				geoDBPath:       filepath.Join(t.TempDir(), "geo.mmdb"),
				client:          &http.Client{Timeout: 2 * time.Second},
				downloadBaseURL: ts.URL,
			}
			u.asnDBPath = u.geoDBPath + "-ASN.mmdb"

			if err := u.Download(); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if called {
				t.Fatal("expected no HTTP request when license key is unusable")
			}
		})
	}
}

// TestDownloadUsesDecryptedKey verifies download() passes the value returned
// by GetMaxMindLicenseKey (the decrypted key) to MaxMind, not the raw
// (possibly encrypted) setting.
func TestDownloadUsesDecryptedKey(t *testing.T) {
	var gotKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("license_key")
		w.WriteHeader(http.StatusBadGateway) // fail fast, we only care about the request
	}))
	defer ts.Close()

	fs := &fakeSettingsStore{licenseKey: "decrypted-plaintext-key"}
	u := &Updater{
		store:           fs,
		resolver:        &Resolver{},
		geoDBPath:       filepath.Join(t.TempDir(), "geo.mmdb"),
		client:          &http.Client{Timeout: 2 * time.Second},
		downloadBaseURL: ts.URL,
	}
	u.asnDBPath = u.geoDBPath + "-ASN.mmdb"

	_ = u.Download() // expected to fail (502), we only assert on the request

	if gotKey != "decrypted-plaintext-key" {
		t.Fatalf("expected decrypted key sent to MaxMind, got %q", gotKey)
	}
}
