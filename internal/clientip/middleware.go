package clientip

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

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

		warner := &warnLimiter{}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(trusted) == 0 || !peerIsTrusted(r, trusted) {
				// An empty set is a deliberate opt-out, so only warn when the
				// operator configured a set that this peer failed to match.
				if len(trusted) > 0 && forwardedHeaderPresent(r) {
					warner.warn(peerHost(r))
				}
				next.ServeHTTP(w, r.WithContext(withClientIP(r.Context(), peerHost(r), false)))
				return
			}
			viaXFF.ServeHTTP(w, r)
		})
	}
}

func forwardedHeaderPresent(r *http.Request) bool {
	return r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Proto") != ""
}

// warnLimiter throttles the untrusted-header warning. A directly-reachable
// instance sees these headers on ordinary scanner traffic, and keying by peer
// would grow without bound, so one throttled line carries the most recent peer
// and how many were suppressed behind it.
type warnLimiter struct {
	mu         sync.Mutex
	last       time.Time
	suppressed int
}

const warnInterval = 10 * time.Minute

func (l *warnLimiter) warn(peer string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.suppressed++
	if !l.last.IsZero() && time.Since(l.last) < warnInterval {
		return
	}
	log.Printf("WARNING: ignoring X-Forwarded-* from untrusted peer %s (%d occurrence(s)); "+
		"if that is your reverse proxy, add it to TRUSTED_PROXIES", peer, l.suppressed)
	l.last = time.Now().UTC()
	l.suppressed = 0
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
