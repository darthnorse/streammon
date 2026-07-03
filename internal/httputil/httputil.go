package httputil

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
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
		return errors.New("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must use http or https scheme")
	}
	if u.Host == "" {
		return errors.New("url must have a host")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		if err := ValidateIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// isBlockedResolvedIP reports whether ip must not be used as an outbound
// connection target. Unlike ValidateIP (which only inspects IP literals
// typed into a config field), this is applied at dial time, after DNS
// resolution — defense-in-depth against SSRF via DNS rebinding, redirects,
// or hostnames that only resolve to an internal address at connection time.
func isBlockedResolvedIP(ip net.IP) bool {
	return ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate()
}

// safeDialControl is a net.Dialer.Control hook that runs after DNS
// resolution but before the socket connects, rejecting resolved addresses
// that are loopback, link-local, private/unique-local, or unspecified.
func safeDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("resolved address %q is not an IP", host)
	}
	if isBlockedResolvedIP(ip) {
		return fmt.Errorf("connection to %s is not allowed", ip)
	}
	return nil
}

// NewSafeClient returns an *http.Client for outbound integration and
// notification requests (e.g. webhook/Discord/ntfy sends). It rejects
// connections whose resolved remote IP is loopback, link-local,
// private/unique-local, or unspecified — even if the URL's hostname looked
// public at config-validation time — and refuses to auto-follow redirects
// so a redirect to an internal host is never dialed (mirrors the pattern in
// overseerr.CreateRequestAsUser).
func NewSafeClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   safeDialControl,
	}).DialContext

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
