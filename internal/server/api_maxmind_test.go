package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
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

// cancelingResolver simulates a client disconnecting partway through a geo
// backfill: after cancelAfter Lookup calls it cancels the request context,
// just as a real client closing the connection would make r.Context() done.
type cancelingResolver struct {
	cancel      context.CancelFunc
	cancelAfter int
	calls       int
	geo         *models.GeoResult
}

func (c *cancelingResolver) Lookup(ip net.IP) *models.GeoResult {
	c.calls++
	if c.calls == c.cancelAfter {
		c.cancel()
	}
	return c.geo
}

// seedUncachedIPs creates a server and one watch_history row per IP so
// GetUncachedIPs(...) returns exactly these IPs.
func seedUncachedIPs(t *testing.T, st *store.Store, ips []string) {
	t.Helper()
	srv := &models.Server{Name: "S", Type: models.ServerTypePlex, URL: "http://s", APIKey: "k"}
	if err := st.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i, ip := range ips {
		entry := models.WatchHistoryEntry{
			ServerID: srv.ID, UserName: "alice", MediaType: "movie",
			Title: fmt.Sprintf("Title %d", i), IPAddress: ip, StartedAt: now, StoppedAt: now,
		}
		if err := st.InsertHistory(&entry); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGeoBackfill_ResolvesAllWhenNotCancelled(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	seedUncachedIPs(t, st, ips)

	resolver := &stubResolver{results: map[string]*models.GeoResult{
		"1.1.1.1": {City: "A", Country: "US"},
		"2.2.2.2": {City: "B", Country: "US"},
		"3.3.3.3": {City: "C", Country: "US"},
	}}
	srv.Unwrap().geoResolver = resolver

	req := httptest.NewRequest(http.MethodPost, "/api/settings/maxmind/backfill", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp geoBackfillResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Resolved != 3 || resp.Total != 3 {
		t.Fatalf("expected resolved=3 total=3, got %+v", resp)
	}
}

// TestGeoBackfill_StopsOnContextCancellation verifies that a client
// disconnect (context cancellation) partway through the backfill stops the
// loop instead of continuing to resolve/cache every remaining IP.
func TestGeoBackfill_StopsOnContextCancellation(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}
	seedUncachedIPs(t, st, ips)

	ctx, cancel := context.WithCancel(context.Background())
	resolver := &cancelingResolver{cancel: cancel, cancelAfter: 2, geo: &models.GeoResult{City: "X", Country: "US"}}
	srv.Unwrap().geoResolver = resolver

	req := httptest.NewRequest(http.MethodPost, "/api/settings/maxmind/backfill", nil).WithContext(ctx)
	addAuthCookie(req)
	w := httptest.NewRecorder()
	srv.Unwrap().ServeHTTP(w, req)

	if resolver.calls >= len(ips) {
		t.Fatalf("expected the loop to stop early on cancellation, but Lookup was called %d times (of %d IPs)", resolver.calls, len(ips))
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected no response body to be written after the client disconnected, got: %s", w.Body.String())
	}
}
