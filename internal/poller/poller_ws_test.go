package poller

import (
	"context"
	"testing"
	"time"

	"streammon/internal/media"
	"streammon/internal/models"
)

type mockRealtimeServer struct {
	mockServer
	ch  chan models.SessionUpdate
	err error
}

func (m *mockRealtimeServer) Subscribe(ctx context.Context) (<-chan models.SessionUpdate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ch, nil
}

// Verify interface compliance
var _ media.RealtimeSubscriber = (*mockRealtimeServer)(nil)

func TestAddServerStartsWebSocket(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ch := make(chan models.SessionUpdate, 8)
	ms := &mockRealtimeServer{
		mockServer: mockServer{
			name: "plex-rt",
			sessions: []models.ActiveStream{
				{SessionID: "s1", ServerID: 1, Title: "Movie", MediaType: models.MediaTypeMovie,
					DurationMs: 100000, ProgressMs: 10000, StartedAt: time.Now().UTC()},
			},
		},
		ch: ch,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	p.AddServer(1, ms)
	triggerAndWaitPoll(t, p)

	// Send a WS update
	ch <- models.SessionUpdate{SessionKey: "s1", State: models.SessionStatePlaying, ViewOffset: 50000}

	// Give the goroutine time to process
	time.Sleep(100 * time.Millisecond)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ProgressMs != 50000 {
		t.Errorf("progress = %d, want 50000", sessions[0].ProgressMs)
	}

	p.Stop()
}

func TestWebSocketStoppedStateRemovesSession(t *testing.T) {
	s := newTestStore(t)

	srv := &models.Server{Name: "srv", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)
	ch := make(chan models.SessionUpdate, 8)
	ms := &mockRealtimeServer{
		mockServer: mockServer{
			name: "plex-rt",
			sessions: []models.ActiveStream{
				{SessionID: "s1", ServerID: srv.ID, Title: "Movie", MediaType: models.MediaTypeMovie,
					DurationMs: 100000, ProgressMs: 50000, UserName: "alice", StartedAt: time.Now().UTC()},
			},
		},
		ch: ch,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	p.AddServer(srv.ID, ms)
	triggerAndWaitPoll(t, p)

	if len(p.CurrentSessions()) != 1 {
		t.Fatal("expected 1 session before stop")
	}

	ch <- models.SessionUpdate{SessionKey: "s1", State: models.SessionStateStopped, ViewOffset: 80000}
	time.Sleep(100 * time.Millisecond)

	if len(p.CurrentSessions()) != 0 {
		t.Errorf("expected 0 sessions after stopped, got %d", len(p.CurrentSessions()))
	}

	result, err := s.ListHistory(1, 10, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 history entry, got %d", result.Total)
	}

	p.Stop()
}

func TestRemoveServerCancelsWebSocket(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ch := make(chan models.SessionUpdate, 8)
	ms := &mockRealtimeServer{
		mockServer: mockServer{
			name: "plex-rt",
			sessions: []models.ActiveStream{
				{SessionID: "s1", ServerID: 1, Title: "Movie"},
			},
		},
		ch: ch,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	p.AddServer(1, ms)
	triggerAndWaitPoll(t, p)

	p.RemoveServer(1)

	// Verify wsCancel entry was removed
	p.mu.RLock()
	_, exists := p.wsCancel[1]
	p.mu.RUnlock()
	if exists {
		t.Error("expected wsCancel entry to be removed")
	}

	p.Stop()
}

func TestSubscribeFailureFallsBackToPolling(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockRealtimeServer{
		mockServer: mockServer{
			name: "plex-rt",
			sessions: []models.ActiveStream{
				{SessionID: "s1", ServerID: 1, Title: "Movie"},
			},
		},
		err: context.DeadlineExceeded,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	p.AddServer(1, ms)
	triggerAndWaitPoll(t, p)

	// Should still have sessions from polling
	if len(p.CurrentSessions()) != 1 {
		t.Fatalf("expected 1 session from polling fallback, got %d", len(p.CurrentSessions()))
	}

	p.Stop()
}
