package httputil

import (
	"strings"
	"testing"
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
