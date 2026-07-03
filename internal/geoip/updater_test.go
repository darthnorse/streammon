package geoip

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDownloadDBRedactsLicenseKeyOnFailure verifies the MaxMind license key
// never appears in the error returned when the download request fails.
// It points downloadDB at a closed local server (connection refused),
// which is exactly the shape of error (*url.Error) MaxMind outages or
// network failures would produce in production.
func TestDownloadDBRedactsLicenseKeyOnFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // force connection refused

	secret := "SUPERSECRETLICENSEKEY"
	u := &Updater{
		client:          &http.Client{Timeout: 2 * time.Second},
		downloadBaseURL: ts.URL,
	}

	destDir := t.TempDir()
	err := u.downloadDB("GeoLite2-City", secret, destDir, filepath.Join(destDir, "out.mmdb"))
	if err == nil {
		t.Fatal("expected an error from a closed server")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("license key leaked into error: %v", err)
	}
}
