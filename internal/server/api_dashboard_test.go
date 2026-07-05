package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
)

func TestDashboardSessionsWithoutPoller(t *testing.T) {
	srv, _ := newTestServerWrapped(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sessions", nil)
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var sessions []models.ActiveStream
	if err := json.NewDecoder(rr.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(sessions))
	}
}

func TestDashboardSessionsWithPoller(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := poller.New(st, time.Hour)
	srv.poller = p

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sessions", nil)
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestSSEWithoutPoller(t *testing.T) {
	srv, _ := newTestServerWrapped(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestSSEStreamsEvents(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := poller.New(st, time.Hour)
	srv.poller = p

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	srv.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(rr.Body.String(), "data:") {
		t.Error("expected at least one SSE data event")
	}
}

// sseTestUser creates a dedicated user + session so cap tests don't share
// a principal with the wrapped testServer's default admin cookie.
func sseTestUser(t *testing.T, st *store.Store, name string) (*models.User, string) {
	t.Helper()
	user, err := st.CreateLocalUser(name, name+"@test.local", "", models.RoleViewer)
	if err != nil {
		t.Fatalf("CreateLocalUser: %v", err)
	}
	token, err := st.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return user, token
}

func TestSSEHandler_RejectsOverCapConnection(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := poller.New(st, time.Hour)
	srv.poller = p
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	user, token := sseTestUser(t, st, "sse-cap-user")
	principal := fmt.Sprintf("user:%d", user.ID)

	s := srv.Unwrap()
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !s.sseConns.tryAcquire(principal) {
			t.Fatalf("setup acquire %d failed", i)
		}
	}
	defer func() {
		for i := 0; i < maxSSEConnsPerPrincipal; i++ {
			s.sseConns.release(principal)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body=%s", rr.Code, rr.Body.String())
	}
	if ra := rr.Header().Get("Retry-After"); ra == "" {
		t.Error("expected Retry-After header on 429 response")
	}
	if ct := rr.Header().Get("Content-Type"); ct == "text/event-stream" {
		t.Error("rejected connection should not carry SSE content-type headers")
	}
}

func TestSSEHandler_ReleasesSlotOnDisconnect(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := poller.New(st, time.Hour)
	srv.poller = p
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	user, token := sseTestUser(t, st, "sse-release-user")
	principal := fmt.Sprintf("user:%d", user.ID)

	s := srv.Unwrap()
	// Simulate cap-1 already-open connections for this principal.
	for i := 0; i < maxSSEConnsPerPrincipal-1; i++ {
		if !s.sseConns.tryAcquire(principal) {
			t.Fatalf("setup acquire %d failed", i)
		}
	}
	defer func() {
		for i := 0; i < maxSSEConnsPerPrincipal-1; i++ {
			s.sseConns.release(principal)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}

	// The handler's defer must have released its slot by the time
	// ServeHTTP returns (request context already expired above).
	s.sseConns.mu.Lock()
	count := s.sseConns.counts[principal]
	s.sseConns.mu.Unlock()
	if count != maxSSEConnsPerPrincipal-1 {
		t.Errorf("count after disconnect = %d, want %d (slot not released)", count, maxSSEConnsPerPrincipal-1)
	}
}

func TestSSEHandler_DifferentPrincipalsIndependent(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	p := poller.New(st, time.Hour)
	srv.poller = p
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	userA, _ := sseTestUser(t, st, "sse-principal-a")
	_, tokenB := sseTestUser(t, st, "sse-principal-b")
	principalA := fmt.Sprintf("user:%d", userA.ID)

	s := srv.Unwrap()
	// Fill principal A's cap fully.
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !s.sseConns.tryAcquire(principalA) {
			t.Fatalf("setup acquire %d failed", i)
		}
	}
	defer func() {
		for i := 0; i < maxSSEConnsPerPrincipal; i++ {
			s.sseConns.release(principalA)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tokenB})
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer reqCancel()
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("principal B status = %d, want 200 (independent of A's cap); body=%s", rr.Code, rr.Body.String())
	}
}
