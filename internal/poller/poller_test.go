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

func (m *mockServer) Name() string             { return m.name }
func (m *mockServer) Type() models.ServerType  { return models.ServerTypePlex }
func (m *mockServer) TestConnection(ctx context.Context) error { return nil }
func (m *mockServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions, m.err
}
func (m *mockServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	return nil, nil
}
func (m *mockServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockServer) ServerID() int64 { return 1 }
func (m *mockServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockServer) DeleteItem(ctx context.Context, itemID string) error {
	return nil
}
func (m *mockServer) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	return nil, nil
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

func newTestStoreWithServer(t *testing.T) (*store.Store, *models.Server) {
	t.Helper()
	s := newTestStore(t)
	srv := &models.Server{Name: "srv", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	return s, srv
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
			{SessionID: "s1", ServerID: 1, Title: "Movie", MediaType: models.MediaTypeMovie, StartedAt: time.Now().UTC()},
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
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, Title: "Movie", MediaType: models.MediaTypeMovie,
				DurationMs: 100000, ProgressMs: 50000, UserName: "alice", StartedAt: time.Now().UTC()},
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

	result, err := s.ListHistory(1, 10, "", "", "", nil)
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

	startTime := time.Now().UTC().Add(-10 * time.Minute)
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
		{SessionID: "s1", ServerID: 1, Title: "Movie", ProgressMs: 5000, StartedAt: time.Now().UTC()},
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
	result, err := s.ListHistory(1, 10, "", "", "", nil)
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

func TestCompoundSessionKey(t *testing.T) {
	s := newTestStore(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: 1, ItemID: "100", Title: "Movie A", MediaType: models.MediaTypeMovie, StartedAt: time.Now().UTC()},
			{SessionID: "s1", ServerID: 1, ItemID: "200", Title: "Movie B", MediaType: models.MediaTypeMovie, StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(1, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (compound key), got %d", len(sessions))
	}

	p.Stop()
}

func TestRatingKeyChangeCreatesHistory(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	// Poll 1: session s1 watching item 100
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Episode 1",
				MediaType: models.MediaTypeTV, DurationMs: 60000, ProgressMs: 55000,
				UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Poll 2: same session s1 but now watching item 200 (autoplay)
	ms.setSessions([]models.ActiveStream{
		{SessionID: "s1", ServerID: srv.ID, ItemID: "200", Title: "Episode 2",
			MediaType: models.MediaTypeTV, DurationMs: 60000, ProgressMs: 5000,
			UserName: "alice", StartedAt: time.Now().UTC()},
	})
	triggerAndWaitPoll(t, p)

	p.Stop()

	// Should have 1 history entry for Episode 1 (the old item)
	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry for autoplay, got %d", result.Total)
	}
	if result.Items[0].Title != "Episode 1" {
		t.Errorf("expected Episode 1, got %s", result.Items[0].Title)
	}

	// Should still have 1 active session for Episode 2
	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].Title != "Episode 2" {
		t.Errorf("expected Episode 2 active, got %s", sessions[0].Title)
	}
}

func TestNearEndWatched(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	// Session stops 5 seconds before end
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 120000, ProgressMs: 115000,
				UserName: "alice", StartedAt: time.Now().UTC()},
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

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry, got %d", result.Total)
	}
	// Within 10s of end → progressMs should be set to durationMs
	if result.Items[0].WatchedMs != 120000 {
		t.Errorf("near-end: watched_ms = %d, want 120000 (duration)", result.Items[0].WatchedMs)
	}
}

func TestNearEndNotTriggered(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 120000, ProgressMs: 100000,
				UserName: "alice", StartedAt: time.Now().UTC()},
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

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Items[0].WatchedMs != 100000 {
		t.Errorf("not near-end: watched_ms = %d, want 100000", result.Items[0].WatchedMs)
	}
}

func TestDLNADebounce(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	// Poll 1: DLNA session appears
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "dlna1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				Platform: "DLNA", MediaType: models.MediaTypeMovie, DurationMs: 60000,
				ProgressMs: 1000, UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Session should be in pending, not active yet
	sessions := p.CurrentSessions()
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions (DLNA pending), got %d", len(sessions))
	}

	// Poll 2: DLNA session still present → promote to real session
	triggerAndWaitPoll(t, p)
	sessions = p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after DLNA promotion, got %d", len(sessions))
	}

	// Poll 3: DLNA session disappears → should create history
	ms.setSessions(nil)
	triggerAndWaitPoll(t, p)
	p.Stop()

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry for promoted DLNA session, got %d", result.Total)
	}
}

func TestDLNATransientNoHistory(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	// Poll 1: DLNA session appears
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "dlna1", ServerID: srv.ID, ItemID: "100", Title: "Browsing",
				Platform: "DLNA", MediaType: models.MediaTypeMovie, DurationMs: 60000,
				ProgressMs: 0, UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Poll 2: DLNA session gone (transient)
	ms.setSessions(nil)
	triggerAndWaitPoll(t, p)
	p.Stop()

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 0 {
		t.Fatalf("expected 0 history entries for transient DLNA session, got %d", result.Total)
	}
}

func TestWatchedThreshold(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Full Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 100000, ProgressMs: 90000,
				UserName: "alice", StartedAt: time.Now().UTC()},
			{SessionID: "s2", ServerID: srv.ID, ItemID: "200", Title: "Abandoned Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 100000, ProgressMs: 30000,
				UserName: "bob", StartedAt: time.Now().UTC()},
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

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 history entries, got %d", result.Total)
	}

	watched := map[string]bool{}
	for _, item := range result.Items {
		watched[item.Title] = item.Watched
	}
	// 90000/100000 = 90% > 85% threshold → watched
	if !watched["Full Movie"] {
		t.Error("expected Full Movie to be marked as watched (90%)")
	}
	// 30000/100000 = 30% < 85% threshold → not watched
	if watched["Abandoned Movie"] {
		t.Error("expected Abandoned Movie to NOT be marked as watched (30%)")
	}
}

func TestCustomWatchedThreshold(t *testing.T) {
	s, srv := newTestStoreWithServer(t)

	// Set threshold to 50%
	if err := s.SetWatchedThreshold(50); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 100000, ProgressMs: 55000,
				UserName: "alice", StartedAt: time.Now().UTC()},
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

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("expected 1 entry, got %d", result.Total)
	}
	// 55000/100000 = 55% > 50% custom threshold → watched
	if !result.Items[0].Watched {
		t.Error("expected watched=true with 50% custom threshold")
	}
}

func TestUpdatePauseState(t *testing.T) {
	s := models.ActiveStream{State: models.SessionStatePlaying}

	// Transition to paused
	updatePauseState(&s, s.State, models.SessionStatePaused)
	if s.State != models.SessionStatePaused {
		t.Errorf("expected paused, got %s", s.State)
	}
	if s.LastPausedAt.IsZero() {
		t.Error("expected LastPausedAt to be set")
	}

	// Simulate time passing
	s.LastPausedAt = time.Now().UTC().Add(-2 * time.Second)

	// Transition back to playing
	updatePauseState(&s, s.State, models.SessionStatePlaying)
	if s.State != models.SessionStatePlaying {
		t.Errorf("expected playing, got %s", s.State)
	}
	if s.PausedMs < 1000 {
		t.Errorf("expected PausedMs >= 1000, got %d", s.PausedMs)
	}
	if !s.LastPausedAt.IsZero() {
		t.Error("expected LastPausedAt to be reset")
	}
}

func TestPausedMsPersistedToHistory(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	p := newTestPoller(t, s)

	// Poll 1: session playing
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 120000, ProgressMs: 10000,
				UserName: "alice", StartedAt: time.Now().UTC(),
				State: models.SessionStatePlaying},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Poll 2: session paused
	ms.setSessions([]models.ActiveStream{
		{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
			MediaType: models.MediaTypeMovie, DurationMs: 120000, ProgressMs: 30000,
			UserName: "alice", StartedAt: time.Now().UTC(),
			State: models.SessionStatePaused},
	})
	triggerAndWaitPoll(t, p)

	// Verify session is now paused with LastPausedAt set
	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].State != models.SessionStatePaused {
		t.Errorf("expected paused state, got %s", sessions[0].State)
	}

	// Poll 3: session resumes then stops
	ms.setSessions(nil)
	triggerAndWaitPoll(t, p)
	p.Stop()

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry, got %d", result.Total)
	}
	// PausedMs should be >= 0 (time spent paused between polls; may be 0 if polls are instant)
	if result.Items[0].PausedMs < 0 {
		t.Errorf("expected non-negative paused_ms, got %d", result.Items[0].PausedMs)
	}
	// Verify the field was actually written (not left as default -1 or similar)
	if result.Items[0].PausedMs > 60000 {
		t.Errorf("paused_ms unreasonably large: %d", result.Items[0].PausedMs)
	}
}

func TestSessionKeyFormat(t *testing.T) {
	key := sessionKey(42, "abc", "100")
	if key != "42:abc:100" {
		t.Errorf("sessionKey = %q, want 42:abc:100", key)
	}
}

func TestSessionPrefix(t *testing.T) {
	prefix := sessionPrefix(42, "abc")
	if prefix != "42:abc:" {
		t.Errorf("sessionPrefix = %q, want 42:abc:", prefix)
	}
}

func TestIsDLNA(t *testing.T) {
	tests := []struct {
		platform string
		player   string
		want     bool
	}{
		{"DLNA", "LG TV", true},
		{"dlna", "LG TV", true},
		{"Chrome", "Plex Web", false},
		{"iOS", "DLNA Player", true},
		{"Android", "Plex for Android", false},
	}
	for _, tt := range tests {
		s := models.ActiveStream{Platform: tt.platform, Player: tt.player}
		if got := isDLNA(s); got != tt.want {
			t.Errorf("isDLNA(%q, %q) = %v, want %v", tt.platform, tt.player, got, tt.want)
		}
	}
}

func TestIdleTimeoutTerminatesSession(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	// Set idle timeout to 1 minute for faster testing
	if err := s.SetIdleTimeoutMinutes(1); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Idle Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Session should be active
	if len(p.CurrentSessions()) != 1 {
		t.Fatalf("expected 1 session, got %d", len(p.CurrentSessions()))
	}

	// Manually set LastProgressChange to 2 minutes ago to simulate idle
	p.mu.Lock()
	for key, sess := range p.sessions {
		sess.LastProgressChange = time.Now().UTC().Add(-2 * time.Minute)
		p.sessions[key] = sess
	}
	p.mu.Unlock()

	// Next poll: same progress (50000) — should trigger idle timeout
	triggerAndWaitPoll(t, p)

	// Session should be gone (idle timeout)
	sessions := p.CurrentSessions()
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after idle timeout, got %d", len(sessions))
	}

	p.Stop()

	// History should have been created
	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry from idle timeout, got %d", result.Total)
	}
	if result.Items[0].Title != "Idle Movie" {
		t.Errorf("expected Idle Movie, got %s", result.Items[0].Title)
	}
}

func TestIdleTimeoutResetsOnProgress(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	if err := s.SetIdleTimeoutMinutes(1); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Active Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Advance progress — this should reset the idle timer
	ms.setSessions([]models.ActiveStream{
		{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Active Movie",
			MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 60000,
			UserName: "alice", StartedAt: time.Now().UTC()},
	})
	triggerAndWaitPoll(t, p)

	// Session should still be active
	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after progress advance, got %d", len(sessions))
	}

	p.Stop()

	// No history should have been created
	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 0 {
		t.Errorf("expected 0 history entries, got %d", result.Total)
	}
}

func TestIdleTimeoutDisabledWhenZero(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	if err := s.SetIdleTimeoutMinutes(0); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Manually set LastProgressChange to way in the past
	p.mu.Lock()
	for key, sess := range p.sessions {
		sess.LastProgressChange = time.Now().UTC().Add(-1 * time.Hour)
		p.sessions[key] = sess
	}
	p.mu.Unlock()

	// Poll again — session should NOT be terminated since idle timeout is disabled
	triggerAndWaitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (idle timeout disabled), got %d", len(sessions))
	}

	p.Stop()
}

func TestIdleTimeoutNewSessionGetsFullWindow(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	if err := s.SetIdleTimeoutMinutes(1); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "New Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Immediately poll again with same progress — should NOT trigger idle
	// because the session just appeared and got LastProgressChange = now
	triggerAndWaitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (new session has full window), got %d", len(sessions))
	}

	p.Stop()
}

func TestIdleTimeoutExemptsPausedSessions(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	if err := s.SetIdleTimeoutMinutes(1); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	// Start with a playing session
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Paused Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC(),
				State: models.SessionStatePlaying},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Transition to paused with same progress
	ms.setSessions([]models.ActiveStream{
		{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Paused Movie",
			MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
			UserName: "alice", StartedAt: time.Now().UTC(),
			State: models.SessionStatePaused},
	})
	triggerAndWaitPoll(t, p)

	// Set LastProgressChange to well past the timeout
	p.mu.Lock()
	for key, sess := range p.sessions {
		sess.LastProgressChange = time.Now().UTC().Add(-10 * time.Minute)
		p.sessions[key] = sess
	}
	p.mu.Unlock()

	// Poll again — paused session should NOT be idle-terminated
	triggerAndWaitPoll(t, p)

	sessions := p.CurrentSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (paused exempt from idle), got %d", len(sessions))
	}
	if sessions[0].State != models.SessionStatePaused {
		t.Errorf("expected paused state, got %s", sessions[0].State)
	}

	p.Stop()
}

func TestIdleTimeoutSetsAccurateStoppedAt(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	if err := s.SetIdleTimeoutMinutes(1); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)

	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, ItemID: "100", Title: "Idle Movie",
				MediaType: models.MediaTypeMovie, DurationMs: 7200000, ProgressMs: 50000,
				UserName: "alice", StartedAt: time.Now().UTC().Add(-30 * time.Minute)},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	// Set LastProgressChange to 5 minutes ago
	progressTime := time.Now().UTC().Add(-5 * time.Minute)
	p.mu.Lock()
	for key, sess := range p.sessions {
		sess.LastProgressChange = progressTime
		p.sessions[key] = sess
	}
	p.mu.Unlock()

	triggerAndWaitPoll(t, p)
	p.Stop()

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 history entry, got %d", result.Total)
	}

	// StoppedAt should be close to progressTime, not now()
	entry := result.Items[0]
	diff := entry.StoppedAt.Sub(progressTime).Abs()
	if diff > 2*time.Second {
		t.Errorf("StoppedAt should be near LastProgressChange (%v), got %v (diff %v)",
			progressTime, entry.StoppedAt, diff)
	}
}
