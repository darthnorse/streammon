package poller

import (
	"context"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestSessionsUseMergedUserName(t *testing.T) {
	s, srv := newTestStoreWithServer(t)
	alice, err := s.GetOrCreateUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.GetOrCreateUser("bob")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.MergeUsers(alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}

	p := newTestPoller(t, s)
	ms := &mockServer{
		name: "test",
		sessions: []models.ActiveStream{
			{SessionID: "s1", ServerID: srv.ID, Title: "Movie", MediaType: models.MediaTypeMovie,
				DurationMs: 100000, ProgressMs: 50000, UserName: "bob", StartedAt: time.Now().UTC()},
		},
	}
	p.AddServer(srv.ID, ms)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	waitPoll(t, p)

	if got := p.CurrentSessions()[0].UserName; got != "alice" {
		t.Errorf("live session user = %q, want alice", got)
	}

	ms.setSessions(nil)
	triggerAndWaitPoll(t, p)
	p.Stop()

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].UserName != "alice" {
		t.Errorf("history = %+v, want one entry for alice", result.Items)
	}
}
