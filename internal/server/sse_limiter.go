package server

import (
	"fmt"
	"net/http"
	"sync"

	"streammon/internal/models"
)

// maxSSEConnsPerPrincipal caps concurrent /api/dashboard/sse connections per
// principal (authenticated user, or client IP as a defensive fallback). This
// stops a single viewer from opening unbounded connections to pin poller
// goroutines/channels and amplify every broadcast to every subscriber.
const maxSSEConnsPerPrincipal = 5

// sseConnLimiter tracks live SSE connection counts per principal.
// Zero value is ready to use.
type sseConnLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

// tryAcquire reports whether principal is under the concurrent-connection
// cap and, if so, reserves a slot for it. Callers that get true back MUST
// call release(principal) exactly once when the connection ends.
func (l *sseConnLimiter) tryAcquire(principal string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[principal] >= maxSSEConnsPerPrincipal {
		return false
	}
	if l.counts == nil {
		l.counts = make(map[string]int)
	}
	l.counts[principal]++
	return true
}

// release frees a slot reserved by a prior successful tryAcquire. It is a
// no-op (not a decrement below zero) if called without a matching acquire.
func (l *sseConnLimiter) release(principal string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[principal] <= 1 {
		delete(l.counts, principal)
		return
	}
	l.counts[principal]--
}

// ssePrincipalKey identifies the caller for connection-cap purposes: the
// authenticated user's ID, or the raw client IP as a fallback for the
// (normally unreachable, since this route is auth-gated) unauthenticated
// case.
func ssePrincipalKey(user *models.User, r *http.Request) string {
	if user != nil {
		return fmt.Sprintf("user:%d", user.ID)
	}
	return "ip:" + rawClientIP(r)
}
