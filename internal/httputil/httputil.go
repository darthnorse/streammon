package httputil

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const DefaultTimeout = 10 * time.Second
const ExtendedTimeout = 15 * time.Second
const IntegrationTimeout = 30 * time.Second
const MaxResponseBody = 2 << 20 // 2 MiB

func NewClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

func NewClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// DrainBody ensures the connection can be reused for keep-alive.
func DrainBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// ValidateIP rejects unspecified and link-local addresses to mitigate SSRF
// (e.g. cloud metadata at 169.254.169.254). Loopback and private IPs are allowed
// since this is a self-hosted application where services commonly run on the same
// host or local network.
func ValidateIP(ip net.IP) error {
	if ip.IsUnspecified() {
		return errors.New("url must not use an unspecified address")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return errors.New("url must not use a link-local address")
	}
	return nil
}

// ValidateIntegrationURL checks that a URL is valid for use as an integration endpoint.
func ValidateIntegrationURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		if err := ValidateIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// Truncate converts a byte slice to string and truncates to maxRunes runes,
// appending "..." if truncated.
func Truncate(b []byte, maxRunes int) string {
	r := []rune(string(b))
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "..."
	}
	return string(r)
}
