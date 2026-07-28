package clientip

import (
	"context"
	"net"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5/middleware"
)

type ctxKey int

const (
	clientIPKey ctxKey = iota
	peerTrustedKey
)

// Middleware resolves the client IP and records whether the request arrived from
// a trusted proxy.
//
// chi's ClientIPFromXFF walks X-Forwarded-For but never inspects RemoteAddr, so
// on its own it lets anyone who can reach this server directly forge their client
// IP. The peer check here is what closes that: the header is consulted only when
// the socket peer is itself trusted.
func Middleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	strs := make([]string, len(trusted))
	for i, p := range trusted {
		strs[i] = p.String()
	}

	return func(next http.Handler) http.Handler {
		// Built once at startup rather than per request. Note the len(trusted)==0
		// guard below is load-bearing: chi's ClientIPFromXFF with no arguments
		// returns the rightmost XFF entry, so reaching it with an empty set would
		// trust the header outright.
		viaXFF := middleware.ClientIPFromXFF(strs...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := middleware.GetClientIP(r.Context())
			if ip == "" {
				ip = peerHost(r)
			}
			next.ServeHTTP(w, r.WithContext(withClientIP(r.Context(), ip, true)))
		}))

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(trusted) == 0 || !peerIsTrusted(r, trusted) {
				next.ServeHTTP(w, r.WithContext(withClientIP(r.Context(), peerHost(r), false)))
				return
			}
			viaXFF.ServeHTTP(w, r)
		})
	}
}

// FromRequest returns the resolved client IP, host only and never empty. Without
// the middleware installed it reports the socket peer.
func FromRequest(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey).(string); ok && ip != "" {
		return ip
	}
	return peerHost(r)
}

// PeerTrusted reports whether the request arrived from a trusted proxy.
func PeerTrusted(r *http.Request) bool {
	trusted, _ := r.Context().Value(peerTrustedKey).(bool)
	return trusted
}

func withClientIP(ctx context.Context, ip string, trusted bool) context.Context {
	ctx = context.WithValue(ctx, clientIPKey, ip)
	return context.WithValue(ctx, peerTrustedKey, trusted)
}

func peerIsTrusted(r *http.Request, trusted []netip.Prefix) bool {
	addr, err := netip.ParseAddr(peerHost(r))
	if err != nil {
		return false
	}
	// Match chi's normalisation so alternate notations cannot alias a trusted IP.
	addr = addr.Unmap().WithZone("")
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func peerHost(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
