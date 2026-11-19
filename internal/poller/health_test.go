package poller

import (
	"context"
	"fmt"
	"testing"
)

func (m *mockServer) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func pollN(p *Poller, n int) {
	for range n {
		p.poll(context.Background())
	}
}

func TestFailingServers_ReportedAfterThreshold(t *testing.T) {
	p := newTestPoller(t, newTestStore(t))
	p.AddServer(3, &mockServer{name: "Jellyfin", err: fmt.Errorf("jellyfin returned status 401")})
	p.AddServer(1, &mockServer{name: "Plex"})

	pollN(p, pollFailureThreshold-1)
	if got := p.FailingServers(); len(got) != 0 {
		t.Fatalf("reported before threshold: %+v", got)
	}

	p.poll(context.Background())
	got := p.FailingServers()
	if len(got) != 1 {
		t.Fatalf("expected 1 failing server, got %+v", got)
	}
	f := got[0]
	if f.ServerID != 3 || f.ServerName != "Jellyfin" || f.Error != "jellyfin returned status 401" {
		t.Errorf("unexpected failure: %+v", f)
	}
	if f.FailingSince.IsZero() || f.FailingSince.Location().String() != "UTC" {
		t.Errorf("FailingSince = %v, want non-zero UTC", f.FailingSince)
	}
}

func TestFailingServers_SinceIsFirstFailureAndErrorIsLatest(t *testing.T) {
	p := newTestPoller(t, newTestStore(t))
	ms := &mockServer{name: "Jellyfin", err: fmt.Errorf("connection refused")}
	p.AddServer(3, ms)

	p.poll(context.Background())
	p.mu.RLock()
	first := p.failures[3].since
	p.mu.RUnlock()

	ms.setErr(fmt.Errorf("status 401"))
	pollN(p, pollFailureThreshold)

	got := p.FailingServers()
	if len(got) != 1 {
		t.Fatalf("expected 1 failing server, got %+v", got)
	}
	if !got[0].FailingSince.Equal(first) {
		t.Errorf("FailingSince = %v, want first failure %v", got[0].FailingSince, first)
	}
	if got[0].Error != "status 401" {
		t.Errorf("Error = %q, want latest error", got[0].Error)
	}
}

func TestFailingServers_ClearedOnSuccess(t *testing.T) {
	p := newTestPoller(t, newTestStore(t))
	ms := &mockServer{name: "Jellyfin", err: fmt.Errorf("boom")}
	p.AddServer(3, ms)
	pollN(p, pollFailureThreshold)

	ms.setErr(nil)
	p.poll(context.Background())
	if got := p.FailingServers(); len(got) != 0 {
		t.Fatalf("expected recovery to clear failure, got %+v", got)
	}

	ms.setErr(fmt.Errorf("boom"))
	pollN(p, pollFailureThreshold-1)
	if got := p.FailingServers(); len(got) != 0 {
		t.Fatalf("failure count not reset by success: %+v", got)
	}
}

func TestFailingServers_ClearedOnRemove(t *testing.T) {
	p := newTestPoller(t, newTestStore(t))
	p.AddServer(3, &mockServer{name: "Jellyfin", err: fmt.Errorf("boom")})
	pollN(p, pollFailureThreshold)

	p.RemoveServer(3)
	if got := p.FailingServers(); len(got) != 0 {
		t.Fatalf("removed server still reported: %+v", got)
	}
}

func TestFailingServers_SortedByServerID(t *testing.T) {
	p := newTestPoller(t, newTestStore(t))
	for _, id := range []int64{5, 2, 9} {
		p.AddServer(id, &mockServer{name: fmt.Sprint(id), err: fmt.Errorf("boom")})
	}
	pollN(p, pollFailureThreshold)

	got := p.FailingServers()
	if len(got) != 3 || got[0].ServerID != 2 || got[1].ServerID != 5 || got[2].ServerID != 9 {
		t.Fatalf("not sorted by server ID: %+v", got)
	}
}
