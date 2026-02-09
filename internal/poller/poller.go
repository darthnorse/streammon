package poller

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	sessions map[string]models.ActiveStream // key: "serverID:sessionID:itemID"

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

	retryMu    sync.Mutex
	retryQueue []retryEntry

	// DLNA sessions must be seen on two consecutive polls before being tracked
	pendingDLNA map[string]models.ActiveStream

	autoLearnHousehold   bool
	autoLearnMinSessions int

	idleTimeout   time.Duration
	idleTimeoutMu sync.RWMutex
}

type retryEntry struct {
	entry    *models.WatchHistoryEntry
	title    string // for logging only
	attempts int
	nextAt   time.Time
}

const (
	maxRetryAttempts = 3
	retryInterval    = 30 * time.Second
)

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
		pendingDLNA: make(map[string]models.ActiveStream),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.RefreshIdleTimeout()
	return p
}

// RefreshIdleTimeout re-reads the idle timeout setting from the store.
// Call after updating the setting via the API.
func (p *Poller) RefreshIdleTimeout() {
	minutes := store.DefaultIdleTimeoutMinutes
	m, err := p.store.GetIdleTimeoutMinutes()
	if err != nil {
		log.Printf("reading idle timeout setting: %v (using default %dm)", err, minutes)
	} else {
		minutes = m
	}
	p.idleTimeoutMu.Lock()
	if minutes > 0 {
		p.idleTimeout = time.Duration(minutes) * time.Minute
	} else {
		p.idleTimeout = 0
	}
	p.idleTimeoutMu.Unlock()
}

func (p *Poller) getIdleTimeout() time.Duration {
	p.idleTimeoutMu.RLock()
	defer p.idleTimeoutMu.RUnlock()
	return p.idleTimeout
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
	var ended []models.ActiveStream
	for key, s := range p.sessions {
		if s.ServerID == id {
			ended = append(ended, s)
			delete(p.sessions, key)
		}
	}
	p.mu.Unlock()
	for _, s := range ended {
		p.persistHistory(s)
	}
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
	prefix := sessionPrefix(serverID, u.SessionKey)

	// Detect rating key change (autoplay) via WebSocket
	for key, session := range p.sessions {
		if session.ServerID == serverID && session.SessionID == u.SessionKey {
			matched = true
			if u.RatingKey != "" && session.ItemID != "" && u.RatingKey != session.ItemID {
				old := session
				delete(p.sessions, key)
				p.mu.Unlock()
				p.persistHistory(old)
				snapshot := p.CurrentSessions()
				p.publish(snapshot)
				return
			}
			ended = p.applySessionChange(key, session, u)
			break
		}
	}

	// Fallback: prefix match for compound key lookups
	if !matched {
		for key, session := range p.sessions {
			if strings.HasPrefix(key, prefix) {
				matched = true
				ended = p.applySessionChange(key, session, u)
				break
			}
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

// applySessionChange handles stop-vs-update for a matched WebSocket session.
// Returns non-nil if the session was stopped. Must be called with p.mu held.
func (p *Poller) applySessionChange(key string, session models.ActiveStream, u models.SessionUpdate) *models.ActiveStream {
	if u.State == models.SessionStateStopped {
		delete(p.sessions, key)
		return &session
	}
	if u.ViewOffset != session.ProgressMs {
		session.LastProgressChange = time.Now().UTC()
	}
	session.ProgressMs = u.ViewOffset
	updatePauseState(&session, session.State, u.State)
	p.sessions[key] = session
	return nil
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

// PersistActiveSessions persists all currently tracked sessions to history
// and clears the session map. Call during graceful shutdown before Stop().
func (p *Poller) PersistActiveSessions() {
	p.mu.Lock()
	sessions := make([]models.ActiveStream, 0, len(p.sessions))
	for _, s := range p.sessions {
		sessions = append(sessions, s)
	}
	p.sessions = make(map[string]models.ActiveStream)
	p.mu.Unlock()
	for _, s := range sessions {
		p.persistHistory(s)
	}
}

type serverEntry struct {
	id          int64
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
	pendingDLNA := make(map[string]models.ActiveStream, len(p.pendingDLNA))
	for k, v := range p.pendingDLNA {
		pendingDLNA[k] = v
	}
	p.mu.RUnlock()

	failedServers := make(map[int64]struct{})
	newSessions := make(map[string]models.ActiveStream)

	seenDLNA := make(map[string]struct{})
	now := time.Now().UTC()
	for _, entry := range servers {
		streams, err := entry.mediaServer.GetSessions(ctx)
		if err != nil {
			log.Printf("polling %s: %v", entry.mediaServer.Name(), err)
			failedServers[entry.id] = struct{}{}
			continue
		}
		for _, s := range streams {
			// DLNA debounce — new DLNA sessions go to pending first
			if isDLNA(s) {
				dlnaKey := sessionKey(s.ServerID, s.SessionID, s.ItemID)
				seenDLNA[dlnaKey] = struct{}{}
				if _, wasTracked := oldSessions[dlnaKey]; !wasTracked {
					if _, wasPending := pendingDLNA[dlnaKey]; !wasPending {
						pendingDLNA[dlnaKey] = s
						continue
					}
				}
				// Was pending or already tracked — promote
				delete(pendingDLNA, dlnaKey)
			}

			key := sessionKey(s.ServerID, s.SessionID, s.ItemID)

			// Detect rating key change (autoplay) — persist old entry
			prefix := sessionPrefix(s.ServerID, s.SessionID)
			for oldKey := range oldSessions {
				if strings.HasPrefix(oldKey, prefix) && oldKey != key {
					p.mu.Lock()
					currentSession, stillActive := p.sessions[oldKey]
					if stillActive {
						delete(p.sessions, oldKey)
					}
					p.mu.Unlock()
					if stillActive {
						p.persistHistory(currentSession)
					}
					delete(oldSessions, oldKey)
					break
				}
			}

			if prev, ok := oldSessions[key]; ok {
				s.StartedAt = prev.StartedAt
				s.PausedMs = prev.PausedMs
				s.LastPausedAt = prev.LastPausedAt

				if s.ProgressMs != prev.ProgressMs {
					s.LastProgressChange = now
				} else {
					s.LastProgressChange = prev.LastProgressChange
				}

				updatePauseState(&s, prev.State, s.State)

				// Log mid-stream quality switches (e.g. bandwidth adaptation)
				if prev.TranscodeKey != "" && s.TranscodeKey != "" && prev.TranscodeKey != s.TranscodeKey {
					log.Printf("transcode key changed for %s: %s -> %s", s.Title, prev.TranscodeKey, s.TranscodeKey)
				}
			} else {
				updatePauseState(&s, "", s.State)
				s.LastProgressChange = now
			}
			s.LastPollSeen = now
			newSessions[key] = s
		}
	}

	for dk := range pendingDLNA {
		if _, seen := seenDLNA[dk]; !seen {
			delete(pendingDLNA, dk)
		}
	}

	preserveFailedSessions(oldSessions, newSessions, failedServers, now.Add(-5*time.Minute))

	var idleStopped []models.ActiveStream
	if idleTimeout := p.getIdleTimeout(); idleTimeout > 0 {
		for key, s := range newSessions {
			if s.State == models.SessionStatePaused {
				continue
			}
			if !s.LastProgressChange.IsZero() && now.Sub(s.LastProgressChange) > idleTimeout {
				log.Printf("idle timeout: %s by %s (no progress for %v)", s.Title, s.UserName, now.Sub(s.LastProgressChange))
				s.IdleStopped = true
				idleStopped = append(idleStopped, s)
				delete(newSessions, key)
				delete(oldSessions, key) // prevent double-persist in disappeared loop
			}
		}
	}

	p.mu.Lock()
	p.sessions = newSessions
	p.pendingDLNA = pendingDLNA
	p.mu.Unlock()

	for key, prev := range oldSessions {
		if _, still := newSessions[key]; !still {
			p.persistHistory(prev)
		}
	}

	for _, s := range idleStopped {
		p.persistHistory(s)
	}

	p.processRetries()

	snapshot := p.CurrentSessions()
	p.publish(snapshot)

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
	progressMs := s.ProgressMs

	// Near-end: if within 10s of end, count as complete
	if s.DurationMs > 0 && (s.DurationMs-progressMs) <= 10000 {
		progressMs = s.DurationMs
	}

	// Finalize pause accumulation on stop (with clock-jump clamping)
	if s.State == models.SessionStatePaused && !s.LastPausedAt.IsZero() {
		elapsed := time.Since(s.LastPausedAt).Milliseconds()
		if elapsed > 0 {
			s.PausedMs += elapsed
		}
	}

	var watched bool
	if s.DurationMs > 0 {
		threshold := 85
		if p.store != nil {
			if t, err := p.store.GetWatchedThreshold(); err == nil {
				threshold = t
			}
		}
		watched = progressMs*100 >= s.DurationMs*int64(threshold)
	}

	entry := p.buildHistoryEntry(s, progressMs, watched)
	if err := p.store.InsertHistory(entry); err != nil {
		log.Printf("persisting history for %s: %v (will retry)", s.Title, err)
		p.enqueueRetry(entry, s.Title)
		return
	}

	if p.autoLearnHousehold && s.IPAddress != "" {
		if _, err := p.store.AutoLearnHouseholdLocation(s.UserName, s.IPAddress, p.autoLearnMinSessions); err != nil {
			log.Printf("auto-learn household for %s: %v", s.UserName, err)
		}
	}
}

func (p *Poller) buildHistoryEntry(s models.ActiveStream, progressMs int64, watched bool) *models.WatchHistoryEntry {
	videoDecision := s.VideoDecision
	if videoDecision == "" {
		videoDecision = models.TranscodeDecisionDirectPlay
	}
	audioDecision := s.AudioDecision
	if audioDecision == "" {
		audioDecision = models.TranscodeDecisionDirectPlay
	}
	stoppedAt := time.Now().UTC()
	if s.IdleStopped && !s.LastProgressChange.IsZero() {
		stoppedAt = s.LastProgressChange
	}
	return &models.WatchHistoryEntry{
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
		WatchedMs:         progressMs,
		Player:            s.Player,
		Platform:          s.Platform,
		IPAddress:         s.IPAddress,
		StartedAt:         s.StartedAt,
		StoppedAt:         stoppedAt,
		SeasonNumber:      s.SeasonNumber,
		EpisodeNumber:     s.EpisodeNumber,
		ThumbURL:          s.ThumbURL,
		VideoResolution:   s.VideoResolution,
		TranscodeDecision: videoDecision, // legacy summary field, same as VideoDecision
		VideoCodec:        s.VideoCodec,
		AudioCodec:        s.AudioCodec,
		AudioChannels:     s.AudioChannels,
		Bandwidth:         s.Bandwidth,
		VideoDecision:     videoDecision,
		AudioDecision:     audioDecision,
		TranscodeHWDecode: s.TranscodeHWDecode,
		TranscodeHWEncode: s.TranscodeHWEncode,
		DynamicRange:      s.DynamicRange,
		PausedMs:          s.PausedMs,
		Watched:           watched,
	}
}

func (p *Poller) enqueueRetry(entry *models.WatchHistoryEntry, title string) {
	p.retryMu.Lock()
	defer p.retryMu.Unlock()
	p.retryQueue = append(p.retryQueue, retryEntry{
		entry:    entry,
		title:    title,
		attempts: 1,
		nextAt:   time.Now().UTC().Add(retryInterval),
	})
}

func (p *Poller) processRetries() {
	p.retryMu.Lock()
	queue := p.retryQueue
	p.retryQueue = nil
	p.retryMu.Unlock()

	if len(queue) == 0 {
		return
	}

	now := time.Now().UTC()
	var remaining []retryEntry
	for _, r := range queue {
		if now.Before(r.nextAt) {
			remaining = append(remaining, r)
			continue
		}
		if err := p.store.InsertHistory(r.entry); err != nil {
			log.Printf("retry %d for %s failed: %v", r.attempts, r.title, err)
			r.attempts++
			if r.attempts > maxRetryAttempts {
				log.Printf("dropping history for %s after %d failed attempts", r.title, maxRetryAttempts)
				continue
			}
			r.nextAt = now.Add(retryInterval)
			remaining = append(remaining, r)
			continue
		}
	}

	if len(remaining) > 0 {
		p.retryMu.Lock()
		p.retryQueue = append(p.retryQueue, remaining...)
		p.retryMu.Unlock()
	}
}

func updatePauseState(s *models.ActiveStream, oldState, newState models.SessionState) {
	if oldState == "" {
		oldState = models.SessionStatePlaying
	}
	s.State = newState

	now := time.Now().UTC()
	if newState == models.SessionStatePaused && oldState != models.SessionStatePaused {
		s.LastPausedAt = now
	} else if newState != models.SessionStatePaused && oldState == models.SessionStatePaused {
		if !s.LastPausedAt.IsZero() {
			elapsed := now.Sub(s.LastPausedAt).Milliseconds()
			if elapsed > 0 {
				s.PausedMs += elapsed
			}
			s.LastPausedAt = time.Time{}
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

// preserveFailedSessions keeps sessions from servers that errored during polling,
// so transient failures don't cause false history entries.
func preserveFailedSessions(oldSessions, newSessions map[string]models.ActiveStream, failedServers map[int64]struct{}, staleThreshold time.Time) {
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
}

func sessionKey(serverID int64, sessionID, itemID string) string {
	return fmt.Sprintf("%d:%s:%s", serverID, sessionID, itemID)
}

func sessionPrefix(serverID int64, sessionID string) string {
	return fmt.Sprintf("%d:%s:", serverID, sessionID)
}

func isDLNA(s models.ActiveStream) bool {
	return strings.EqualFold(s.Platform, "DLNA") ||
		strings.Contains(strings.ToLower(s.Player), "dlna")
}
