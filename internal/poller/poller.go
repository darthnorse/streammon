package poller

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/rules"
	"streammon/internal/store"
)

type Poller struct {
	store    *store.Store
	interval time.Duration

	mu       sync.RWMutex
	servers  map[int64]media.MediaServer
	sessions map[string]models.ActiveStream // key: "serverID:sessionID"

	subMu       sync.Mutex
	subscribers map[chan []models.ActiveStream]struct{}

	startOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}

	wsCancel    map[int64]context.CancelFunc
	triggerPoll chan struct{}
	pollNotify  chan struct{}

	rulesEngine *rules.Engine

	// Household auto-learning settings
	autoLearnHousehold    bool
	autoLearnMinSessions  int
}

// DefaultAutoLearnMinSessions is the default threshold for auto-learning household locations.
const DefaultAutoLearnMinSessions = 10

type PollerOption func(*Poller)

func WithRulesEngine(e *rules.Engine) PollerOption {
	return func(p *Poller) {
		p.rulesEngine = e
	}
}

// WithHouseholdAutoLearn enables automatic learning of household locations
// based on IP usage frequency. minSessions is the threshold for auto-learning.
// Pass 0 or negative to disable auto-learning entirely.
// Default threshold is DefaultAutoLearnMinSessions (10 sessions from the same IP).
func WithHouseholdAutoLearn(minSessions int) PollerOption {
	return func(p *Poller) {
		if minSessions <= 0 {
			// Disabled
			p.autoLearnHousehold = false
			p.autoLearnMinSessions = 0
		} else {
			p.autoLearnHousehold = true
			p.autoLearnMinSessions = minSessions
		}
	}
}

func New(s *store.Store, interval time.Duration, opts ...PollerOption) *Poller {
	p := &Poller{
		store:       s,
		interval:    interval,
		servers:     make(map[int64]media.MediaServer),
		sessions:    make(map[string]models.ActiveStream),
		subscribers: make(map[chan []models.ActiveStream]struct{}),
		wsCancel:    make(map[int64]context.CancelFunc),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Poller) AddServer(id int64, ms media.MediaServer) {
	p.mu.Lock()
	p.servers[id] = ms
	ctx := p.ctx
	if rt, ok := ms.(media.RealtimeSubscriber); ok && ctx != nil {
		wsCtx, cancel := context.WithCancel(ctx)
		p.wsCancel[id] = cancel
		p.mu.Unlock()
		go p.consumeUpdates(wsCtx, id, rt)
		return
	}
	p.mu.Unlock()
}

func (p *Poller) RemoveServer(id int64) {
	p.mu.Lock()
	if cancel, ok := p.wsCancel[id]; ok {
		cancel()
		delete(p.wsCancel, id)
	}
	delete(p.servers, id)
	for key, s := range p.sessions {
		if s.ServerID == id {
			delete(p.sessions, key)
		}
	}
	p.mu.Unlock()
}

func (p *Poller) GetServer(id int64) (media.MediaServer, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ms, ok := p.servers[id]
	return ms, ok
}

func (p *Poller) Start(ctx context.Context) {
	p.startOnce.Do(func() {
		ctx, p.cancel = context.WithCancel(ctx)
		p.mu.Lock()
		p.ctx = ctx
		p.mu.Unlock()
		p.done = make(chan struct{})
		go p.run(ctx)
	})
}

func (p *Poller) Stop() {
	if p.cancel != nil && p.done != nil {
		p.cancel()
		<-p.done
	}
}

func (p *Poller) CurrentSessions() []models.ActiveStream {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]models.ActiveStream, 0, len(p.sessions))
	for _, s := range p.sessions {
		result = append(result, s)
	}
	return result
}

func (p *Poller) Subscribe() chan []models.ActiveStream {
	ch := make(chan []models.ActiveStream, 1)
	p.subMu.Lock()
	p.subscribers[ch] = struct{}{}
	p.subMu.Unlock()
	return ch
}

func (p *Poller) Unsubscribe(ch chan []models.ActiveStream) {
	p.subMu.Lock()
	_, exists := p.subscribers[ch]
	delete(p.subscribers, ch)
	p.subMu.Unlock()
	if exists {
		close(ch)
	}
}

func (p *Poller) consumeUpdates(ctx context.Context, serverID int64, rt media.RealtimeSubscriber) {
	ch, err := rt.Subscribe(ctx)
	if err != nil {
		log.Printf("ws subscribe failed for server %d: %v", serverID, err)
		return
	}
	for update := range ch {
		p.applyUpdate(serverID, update)
	}
}

func (p *Poller) applyUpdate(serverID int64, u models.SessionUpdate) {
	p.mu.Lock()
	var ended *models.ActiveStream
	matched := false
	for key, session := range p.sessions {
		if session.ServerID == serverID && session.SessionID == u.SessionKey {
			matched = true
			if u.State == models.SessionStateStopped {
				ended = &session
				delete(p.sessions, key)
			} else {
				session.ProgressMs = u.ViewOffset
				p.sessions[key] = session
			}
			break
		}
	}
	p.mu.Unlock()

	if !matched {
		return
	}

	if ended != nil {
		p.persistHistory(*ended)
	}

	snapshot := p.CurrentSessions()
	p.publish(snapshot)
}

func (p *Poller) run(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		case <-p.triggerPoll:
			p.poll(ctx)
		}
	}
}

type serverEntry struct {
	id int64
	mediaServer media.MediaServer
}

func (p *Poller) poll(ctx context.Context) {
	p.mu.RLock()
	servers := make([]serverEntry, 0, len(p.servers))
	for id, ms := range p.servers {
		servers = append(servers, serverEntry{id: id, mediaServer: ms})
	}
	oldSessions := make(map[string]models.ActiveStream, len(p.sessions))
	for k, v := range p.sessions {
		oldSessions[k] = v
	}
	p.mu.RUnlock()

	failedServers := make(map[int64]struct{})
	newSessions := make(map[string]models.ActiveStream)

	now := time.Now().UTC()
	for _, entry := range servers {
		streams, err := entry.mediaServer.GetSessions(ctx)
		if err != nil {
			log.Printf("polling %s: %v", entry.mediaServer.Name(), err)
			failedServers[entry.id] = struct{}{}
			continue
		}
		for _, s := range streams {
			key := sessionKey(s.ServerID, s.SessionID)
			if prev, ok := oldSessions[key]; ok {
				s.StartedAt = prev.StartedAt
			}
			s.LastPollSeen = now
			newSessions[key] = s
		}
	}

	// Keep recent sessions from failed servers to avoid false history entries
	staleThreshold := now.Add(-5 * time.Minute)
	for key, prev := range oldSessions {
		if _, failed := failedServers[prev.ServerID]; failed {
			if _, exists := newSessions[key]; !exists {
				if prev.LastPollSeen.After(staleThreshold) {
					newSessions[key] = prev
				} else {
					log.Printf("removing stale session %s (last seen %v)", prev.Title, prev.LastPollSeen)
				}
			}
		}
	}

	p.mu.Lock()
	p.sessions = newSessions
	p.mu.Unlock()

	for key, prev := range oldSessions {
		if _, still := newSessions[key]; !still {
			p.persistHistory(prev)
		}
	}

	snapshot := p.CurrentSessions()
	p.publish(snapshot)

	// Evaluate active sessions against rules
	if p.rulesEngine != nil {
		for i := range snapshot {
			p.rulesEngine.EvaluateSession(ctx, &snapshot[i], snapshot)
		}
	}

	if p.pollNotify != nil {
		select {
		case p.pollNotify <- struct{}{}:
		default:
		}
	}
}

func (p *Poller) persistHistory(s models.ActiveStream) {
	// Default to DirectPlay if decision fields are empty
	videoDecision := s.VideoDecision
	if videoDecision == "" {
		videoDecision = models.TranscodeDecisionDirectPlay
	}
	audioDecision := s.AudioDecision
	if audioDecision == "" {
		audioDecision = models.TranscodeDecisionDirectPlay
	}
	entry := &models.WatchHistoryEntry{
		ServerID:          s.ServerID,
		ItemID:            s.ItemID,
		GrandparentItemID: s.GrandparentItemID,
		UserName:          s.UserName,
		MediaType:         s.MediaType,
		Title:             s.Title,
		ParentTitle:       s.ParentTitle,
		GrandparentTitle:  s.GrandparentTitle,
		Year:              s.Year,
		DurationMs:        s.DurationMs,
		WatchedMs:         s.ProgressMs,
		Player:            s.Player,
		Platform:          s.Platform,
		IPAddress:         s.IPAddress,
		StartedAt:         s.StartedAt,
		StoppedAt:         time.Now().UTC(),
		SeasonNumber:      s.SeasonNumber,
		EpisodeNumber:     s.EpisodeNumber,
		ThumbURL:          s.ThumbURL,
		VideoResolution:   s.VideoResolution,
		TranscodeDecision: videoDecision,
		VideoCodec:        s.VideoCodec,
		AudioCodec:        s.AudioCodec,
		AudioChannels:     s.AudioChannels,
		Bandwidth:         s.Bandwidth,
		VideoDecision:     videoDecision,
		AudioDecision:     audioDecision,
		TranscodeHWDecode: s.TranscodeHWDecode,
		TranscodeHWEncode: s.TranscodeHWEncode,
		DynamicRange:      s.DynamicRange,
	}
	if err := p.store.InsertHistory(entry); err != nil {
		log.Printf("persisting history for %s: %v", s.Title, err)
		return
	}

	// Auto-learn household locations if enabled
	if p.autoLearnHousehold && s.IPAddress != "" {
		if _, err := p.store.AutoLearnHouseholdLocation(s.UserName, s.IPAddress, p.autoLearnMinSessions); err != nil {
			log.Printf("auto-learn household for %s: %v", s.UserName, err)
		}
	}
}

func (p *Poller) publish(snapshot []models.ActiveStream) {
	p.subMu.Lock()
	defer p.subMu.Unlock()
	for ch := range p.subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func sessionKey(serverID int64, sessionID string) string {
	return fmt.Sprintf("%d:%s", serverID, sessionID)
}
