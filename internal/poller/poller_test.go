package poller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

type mockServer struct {
	mu       sync.Mutex
	name     string
	sessions []models.ActiveStream
	err      error
}

func (m *mockServer) Name() string            { return m.name }
func (m *mockServer) Type() models.ServerType  { return models.ServerTypePlex }
func (m *mockServer) TestConnection(ctx context.Context) error { return nil }
func (m *mockServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions, m.err
}

func (m *mockServer) setSessions(s []models.ActiveStream) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = s
}

func (m *mockServer) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestPoller(t *testing.T, s *store.Store) *Poller {
	t.Helper()
	p := New(s, time.Hour) // long interval; we trigger polls manually
	p.triggerPoll = make(chan struct{}, 1)
	p.pollNotify = make(chan struct{}, 1)
	return p
}

func waitPoll(t *testing.T, p *Poller) {
	t.Helper()
	select {
	case <-p.pollNotify:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for poll")
	}
}

func triggerAndWaitPoll(t *testing.T, p *Poller) {
	t.Helper()
	p.triggerPoll <- struct{}{}
	waitPoll(t, p)
}

func TestNewSessionAppears(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, Title: "Movie", MediaType: models.MediaTypeMovie, StartedAt: time.Now()},
		},
	}
	p.AddServer(1, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "Movie" {
		t.Errorf("title = %q, want Movie", sessions[0].Title)
	}

	p.Stop()
}

func TestSessionDisappearsCreatesHistory(t *testing.T) {
	s := newTestStore(t)

	srv := &models.Server{Name: "srv", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, Title: "Movie", MediaType: models.MediaTypeMovie,
				DurationMs: 100000, ProgressMs: 50000, UserName: "alice", StartedAt: time.Now()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	ms.setSessions(nil)
	triggerAndWaitPoll(t, p)

	p.Stop()

	result, err := s.ListHistory(1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry, got %d", result.Total)
	}
	if result.Items[0].Title != "Movie" {
		t.Errorf("history title = %q, want Movie", result.Items[0].Title)
	}
	if result.Items[0].WatchedMs != 50000 {
		t.Errorf("watched = %d, want 50000", result.Items[0].WatchedMs)
	}
}

func TestSessionContinuesPreservesStartedAt(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	startTime := time.Now().Add(-10 * time.Minute)
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, Title: "Movie", StartedAt: startTime},
		},
	}
	p.AddServer(1, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	ms.setSessions([]models.ActiveStream{
		{SessionID: "s1", ServerID: 1, Title: "Movie", ProgressMs: 5000, StartedAt: time.Now()},
	})
	triggerAndWaitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].StartedAt.After(startTime.Add(time.Second)) {
		t.Error("StartedAt was not preserved from original session")
	}

	p.Stop()
}

func TestMultipleServers(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms1 := &mockServer{
		name: "srv1",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, Title: "Movie1"},
		},
	}
	ms2 := &mockServer{
		name: "srv2",
		sessions: []models.ActiveStream{
			{SessionID: "s2", ServerID: 2, Title: "Movie2"},
		},
	}
	p.AddServer(1, ms1)
	p.AddServer(2, ms2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	p.Stop()
}

func TestServerErrorPreservesSessions(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "flaky",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, Title: "Movie", MediaType: models.MediaTypeMovie},
		},
	}
	p.AddServer(1, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	if len(p.CurrentSessions()) != 1 {
		t.Fatal("expected 1 session after first poll")
	}

	// Server errors on next poll — sessions should be carried forward, not lost
	ms.setError(fmt.Errorf("connection refused"))
	triggerAndWaitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session preserved after error, got %d", len(sessions))
	}

	// No history should have been created (session was carried forward)
	result, err := s.ListHistory(1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 history entries, got %d", result.Total)
	}

	p.Stop()
}

func TestServerErrorWithNoExistingSessions(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "bad",
		err: fmt.Errorf("connection refused"),
	}
	p.AddServer(1, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	p.Stop()
}

func TestSubscribeReceivesUpdates(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, Title: "Movie"},
		},
	}
	p.AddServer(1, ms)

	ch := p.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	var received []models.ActiveStream
	select {
	case snap := <-ch:
		received = snap
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for subscription update")
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 session from subscription, got %d", len(received))
	}

	p.Unsubscribe(ch)
	p.Stop()
}

func TestDoubleUnsubscribeDoesNotPanic(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ch := p.Subscribe()
	p.Unsubscribe(ch)
	p.Unsubscribe(ch) // should not panic
}
