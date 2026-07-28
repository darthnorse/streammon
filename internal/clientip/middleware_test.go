package clientip

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// serve runs a request through Middleware and reports what the handler observed.
func serve(t *testing.T, trustedRaw, remoteAddr string, xff []string) (ip string, trusted bool) {
	t.Helper()

	prefixes, err := ParseTrustedProxies(trustedRaw, true)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", trustedRaw, err)
	}

	h := Middleware(prefixes)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ip, trusted = FromRequest(r), PeerTrusted(r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return ip, trusted
}

// The gate chi omits: ClientIPFromXFF never inspects RemoteAddr, so without this
// check anyone reaching the port directly could forge their client IP.
func TestMiddleware_UntrustedPeerIgnoresForgedXFF(t *testing.T) {
	ip, trusted := serve(t, "192.168.0.0/16", "203.0.113.9:5555", []string{"1.2.3.4"})

	if ip != "203.0.113.9" {
		t.Errorf("client IP = %q, want the socket peer 203.0.113.9 (forged header must be ignored)", ip)
	}
	if trusted {
		t.Error("PeerTrusted = true for an untrusted peer")
	}
}

func TestMiddleware_TrustedPeerUsesXFF(t *testing.T) {
	ip, trusted := serve(t, "192.168.0.0/16", "192.168.0.24:5555", []string{"203.0.113.9"})

	if ip != "203.0.113.9" {
		t.Errorf("client IP = %q, want 203.0.113.9 from the header", ip)
	}
	if !trusted {
		t.Error("PeerTrusted = false for a trusted peer")
	}
}

func TestMiddleware_TrustedPeerWalksPastTrustedHops(t *testing.T) {
	ip, _ := serve(t, "192.168.0.0/16", "192.168.0.24:5555",
		[]string{"203.0.113.9, 192.168.0.7, 192.168.0.24"})

	if ip != "203.0.113.9" {
		t.Errorf("client IP = %q, want the leftmost untrusted entry 203.0.113.9", ip)
	}
}

func TestMiddleware_TrustedPeerWithoutXFFFallsBackToPeer(t *testing.T) {
	ip, _ := serve(t, "192.168.0.0/16", "192.168.0.24:5555", nil)

	if ip != "192.168.0.24" {
		t.Errorf("client IP = %q, want the peer 192.168.0.24", ip)
	}
}

func TestMiddleware_TrustedPeerWithGarbageXFFFailsClosed(t *testing.T) {
	ip, _ := serve(t, "192.168.0.0/16", "192.168.0.24:5555", []string{"not-an-ip"})

	if ip != "192.168.0.24" {
		t.Errorf("client IP = %q, want the peer 192.168.0.24 on unparseable header", ip)
	}
}

// A v4-mapped IPv6 peer must fold to v4 before the prefix check, or an attacker
// could dodge the trusted set by switching notation.
func TestMiddleware_V4MappedPeerFolds(t *testing.T) {
	ip, trusted := serve(t, "192.168.0.0/16", "[::ffff:192.168.0.24]:5555", []string{"203.0.113.9"})

	if !trusted {
		t.Error("PeerTrusted = false for a v4-mapped trusted peer")
	}
	if ip != "203.0.113.9" {
		t.Errorf("client IP = %q, want 203.0.113.9", ip)
	}
}

func TestMiddleware_EmptyTrustedSetAlwaysUsesPeer(t *testing.T) {
	ip, trusted := serve(t, "none", "192.168.0.24:5555", []string{"1.2.3.4"})

	if ip != "192.168.0.24" {
		t.Errorf("client IP = %q, want the peer when nothing is trusted", ip)
	}
	if trusted {
		t.Error("PeerTrusted = true with an empty trusted set")
	}
}

// No middleware installed at all: accessors must still be safe and truthful.
func TestFromRequest_WithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:5555"

	if got := FromRequest(req); got != "203.0.113.9" {
		t.Errorf("FromRequest = %q, want the peer 203.0.113.9", got)
	}
	if PeerTrusted(req) {
		t.Error("PeerTrusted = true without the middleware installed")
	}
}

// captureLog swaps the standard logger's output for the duration of fn.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	fn()
	return buf.String()
}

// A misconfigured TRUSTED_PROXIES silently discards X-Forwarded-*; without a log
// line the operator only discovers it by inspecting cookies in devtools.
func TestMiddleware_WarnsOnForwardedHeadersFromUntrustedPeer(t *testing.T) {
	out := captureLog(t, func() {
		serve(t, "192.168.0.0/16", "203.0.113.9:5555", []string{"1.2.3.4"})
	})

	if !strings.Contains(out, "203.0.113.9") {
		t.Errorf("warning should name the untrusted peer, got %q", out)
	}
	if !strings.Contains(out, "TRUSTED_PROXIES") {
		t.Errorf("warning should point at the setting to change, got %q", out)
	}
}

func TestMiddleware_NoWarnForTrustedPeerOrExplicitOptOut(t *testing.T) {
	trusted := captureLog(t, func() {
		serve(t, "192.168.0.0/16", "192.168.0.24:5555", []string{"203.0.113.9"})
	})
	if trusted != "" {
		t.Errorf("no warning expected for a trusted peer, got %q", trusted)
	}

	// TRUSTED_PROXIES=none is a deliberate choice; warning every interval is noise.
	optOut := captureLog(t, func() {
		serve(t, "none", "203.0.113.9:5555", []string{"1.2.3.4"})
	})
	if optOut != "" {
		t.Errorf("no warning expected when trust is explicitly disabled, got %q", optOut)
	}
}

// A directly-exposed instance sees scanner traffic carrying these headers; the
// warning must not turn that into a log flood.
func TestMiddleware_WarnIsThrottled(t *testing.T) {
	prefixes, err := ParseTrustedProxies("192.168.0.0/16", true)
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	h := Middleware(prefixes)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	out := captureLog(t, func() {
		for i := 0; i < 50; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "203.0.113.9:5555"
			req.Header.Set("X-Forwarded-For", "1.2.3.4")
			h.ServeHTTP(httptest.NewRecorder(), req)
		}
	})

	if got := strings.Count(out, "TRUSTED_PROXIES"); got != 1 {
		t.Errorf("50 requests produced %d warnings, want exactly 1", got)
	}
}
