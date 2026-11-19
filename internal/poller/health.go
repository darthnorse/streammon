package poller

import (
	"sort"
	"time"

	"streammon/internal/media"
	"streammon/internal/models"
)

// pollFailureThreshold is how many consecutive failed polls it takes before a
// server is reported as failing, so a single network blip doesn't raise an alert.
const pollFailureThreshold = 3

type pollFailure struct {
	since time.Time
	count int
	err   string
}

// recordPollResult drops results from a client that was removed or replaced
// while its poll was in flight, so a stale error never lands on the new config.
func (p *Poller) recordPollResult(serverID int64, ms media.MediaServer, err error, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cur, ok := p.servers[serverID]; !ok || cur != ms {
		return
	}
	if err == nil {
		delete(p.failures, serverID)
		return
	}
	f, ok := p.failures[serverID]
	if !ok {
		f.since = now
	}
	f.count++
	f.err = err.Error()
	p.failures[serverID] = f
}

// FailingServers returns the servers whose last pollFailureThreshold polls all
// failed, ordered by server ID. While a server is failing, new playback on it
// goes unseen, so its watch history may be incomplete.
func (p *Poller) FailingServers() []models.ServerPollFailure {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := []models.ServerPollFailure{}
	for id, f := range p.failures {
		ms, ok := p.servers[id]
		if !ok || f.count < pollFailureThreshold {
			continue
		}
		out = append(out, models.ServerPollFailure{
			ServerID:     id,
			ServerName:   ms.Name(),
			Error:        f.err,
			FailingSince: f.since,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServerID < out[j].ServerID })
	return out
}
