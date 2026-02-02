package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/poller"
)

func TestDashboardSessionsWithoutPoller(t *testing.T) {
	srv, _ := newTestServer(t)
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
	srv, st := newTestServer(t)
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
	srv, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/sse", nil)
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestSSEStreamsEvents(t *testing.T) {
	srv, st := newTestServer(t)
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
