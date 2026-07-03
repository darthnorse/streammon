package httputil

import (
	"net"
	"net/http"
	"net/http/httptest"
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
