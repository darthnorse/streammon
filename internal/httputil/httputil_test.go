package httputil

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateIntegrationURL(t *testing.T) {
	valid := []string{
		"http://overseerr.example.com",
		"https://plex.example.com:32400",
		"http://media.lan:8096",
		"https://10.0.0.1:8080",
		"http://192.168.1.100:32400",
		"http://127.0.0.1:8080",
		"https://[::1]:443",
	}
	for _, u := range valid {
		if err := ValidateIntegrationURL(u); err != nil {
			t.Errorf("expected %q to be valid, got: %v", u, err)
		}
	}

	invalid := []struct {
		url    string
		errMsg string
	}{
		{"", "required"},
		{"ftp://example.com", "http or https"},
		{"http://", "host"},
		{"not-a-url", "http or https"},
		{"http://0.0.0.0", "unspecified"},
		{"http://[::]:8080", "unspecified"},
		{"http://169.254.1.1", "link-local"},
		{"http://[fe80::1]", "link-local"},
	}
	for _, tc := range invalid {
		err := ValidateIntegrationURL(tc.url)
		if err == nil {
			t.Errorf("expected %q to be invalid", tc.url)
			continue
		}
		if !strings.Contains(strings.ToLower(err.Error()), tc.errMsg) {
			t.Errorf("expected error for %q to contain %q, got: %v", tc.url, tc.errMsg, err)
		}
	}
}

func TestIsBlockedResolvedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback (v6)
		"169.254.1.1",     // link-local
		"169.254.169.254", // link-local (cloud metadata)
		"fe80::1",         // link-local (v6)
		"0.0.0.0",         // unspecified
		"::",              // unspecified (v6)
	}
	for _, s := range blocked {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("test bug: %q did not parse as an IP", s)
		}
		if !isBlockedResolvedIP(ip) {
			t.Errorf("expected %s to be blocked", s)
		}
	}

	allowed := []string{
		"8.8.8.8",              // public
		"1.1.1.1",              // public
		"2606:4700:4700::1111", // public v6 (cloudflare)
		"10.0.0.1",             // private (LAN) — allowed
		"172.16.0.1",           // private (LAN) — allowed
		"192.168.1.1",          // private (LAN) — allowed
		"fc00::1",              // unique-local (ULA) — allowed
	}
	for _, s := range allowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("test bug: %q did not parse as an IP", s)
		}
		if isBlockedResolvedIP(ip) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

func TestNewSafeClient_BlocksLoopbackConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSafeClient(2 * time.Second)
	resp, err := client.Get(server.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected connection to loopback address to be refused")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected error to indicate the dial guard refused the connection, got: %v", err)
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name:        "maxmind license_key",
			in:          "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=SECRET123&suffix=tar.gz",
			wantAbsent:  []string{"SECRET123"},
			wantPresent: []string{"download.maxmind.com", "edition_id=GeoLite2-City", "license_key=REDACTED"},
		},
		{
			name:        "tmdb api_key",
			in:          "https://api.themoviedb.org/3/configuration?api_key=SECRETTMDB",
			wantAbsent:  []string{"SECRETTMDB"},
			wantPresent: []string{"api.themoviedb.org", "api_key=REDACTED"},
		},
		{
			name:        "tautulli apikey",
			in:          "http://localhost:8181/api/v2?apikey=SECRETTAUTULLI&cmd=get_history",
			wantAbsent:  []string{"SECRETTAUTULLI"},
			wantPresent: []string{"cmd=get_history", "apikey=REDACTED"},
		},
		{
			name:        "no query string is unchanged",
			in:          "https://example.com/path",
			wantPresent: []string{"https://example.com/path"},
		},
		{
			name:        "no sensitive params is unchanged",
			in:          "https://example.com/path?foo=bar",
			wantPresent: []string{"foo=bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactURL(tt.in)
			for _, s := range tt.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("RedactURL(%q) = %q, expected %q to be absent", tt.in, got, s)
				}
			}
			for _, s := range tt.wantPresent {
				if !strings.Contains(got, s) {
					t.Errorf("RedactURL(%q) = %q, expected %q to be present", tt.in, got, s)
				}
			}
		})
	}
}

func TestRedactURLError_RedactsURLErrorLicenseKey(t *testing.T) {
	secret := "SECRETVALUE123"
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=" + secret + "&suffix=tar.gz",
		Err: errors.New("connection refused"),
	}

	got := RedactURLError(urlErr)
	if got == nil {
		t.Fatal("expected non-nil error")
	}
	if strings.Contains(got.Error(), secret) {
		t.Errorf("secret leaked in redacted error: %v", got)
	}
	if !strings.Contains(got.Error(), "connection refused") {
		t.Errorf("expected underlying cause preserved, got: %v", got)
	}
	if !strings.Contains(got.Error(), "download.maxmind.com") {
		t.Errorf("expected host preserved, got: %v", got)
	}

	var redactedURLErr *url.Error
	if !errors.As(got, &redactedURLErr) {
		t.Errorf("expected result to still be a *url.Error, got %T", got)
	}
}

func TestRedactURLError_ScrubsWrappedErrorMessage(t *testing.T) {
	secret := "SECRETVALUE123"
	inner := &url.Error{
		Op:  "Get",
		URL: "https://api.themoviedb.org/3/configuration?api_key=" + secret,
		Err: errors.New("connection refused"),
	}
	wrapped := fmt.Errorf("connection failed: %w", inner)

	got := RedactURLError(wrapped)
	if strings.Contains(got.Error(), secret) {
		t.Errorf("secret leaked in redacted wrapped error: %v", got)
	}
	if !strings.Contains(got.Error(), "connection failed") {
		t.Errorf("expected wrapping context preserved, got: %v", got)
	}
}

func TestRedactURLError_PlainErrorPassesThrough(t *testing.T) {
	err := errors.New("some unrelated failure")
	got := RedactURLError(err)
	if got.Error() != err.Error() {
		t.Errorf("expected plain error unchanged, got: %v", got)
	}
}

func TestRedactURLError_Nil(t *testing.T) {
	if got := RedactURLError(nil); got != nil {
		t.Errorf("expected nil, got: %v", got)
	}
}

func TestNewSafeClient_DoesNotFollowRedirects(t *testing.T) {
	client := NewSafeClient(2 * time.Second)
	if client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to be set")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
	via, _ := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	err := client.CheckRedirect(req, []*http.Request{via})
	if err != http.ErrUseLastResponse {
		t.Errorf("expected CheckRedirect to return http.ErrUseLastResponse, got: %v", err)
	}
}
