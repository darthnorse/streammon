package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/store"
)

// countingEngineStore wraps an EngineStore to count the per-tick-constant reads,
// so tests can assert they happen once per tick rather than once per session.
type countingEngineStore struct {
	EngineStore
	unitCalls      atomic.Int32
	householdCalls atomic.Int32
}

func (c *countingEngineStore) GetUnitSystem() (string, error) {
	c.unitCalls.Add(1)
	return c.EngineStore.GetUnitSystem()
}

func (c *countingEngineStore) ListTrustedHouseholdLocations(userName string) ([]models.HouseholdLocation, error) {
	c.householdCalls.Add(1)
	return c.EngineStore.ListTrustedHouseholdLocations(userName)
}

// TestEngine_EvaluateSessions_HoistsConstantReads verifies the batched tick path
// reads the unit system once per tick (not per session) and the trusted
// households once per distinct user (not per session).
func TestEngine_EvaluateSessions_HoistsConstantReads(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	// A high limit so no violation fires; households/unit are still read.
	config := models.ConcurrentStreamsConfig{MaxStreams: 100}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{Name: "loose", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: configJSON}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	e.RefreshRules()

	cs := &countingEngineStore{EngineStore: e.store}
	e.store = cs

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "alice", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "alice", IPAddress: "192.168.1.2", StartedAt: now},
		{SessionID: "c", UserName: "alice", IPAddress: "192.168.1.3", StartedAt: now},
		{SessionID: "d", UserName: "bob", IPAddress: "192.168.1.4", StartedAt: now},
	}

	e.EvaluateSessions(ctx, streams)

	if got := cs.unitCalls.Load(); got != 1 {
		t.Errorf("GetUnitSystem calls = %d, want 1 (hoisted once per tick)", got)
	}
	// Two distinct users -> two household reads (cached per user within the tick).
	if got := cs.householdCalls.Load(); got != 2 {
		t.Errorf("ListTrustedHouseholdLocations calls = %d, want 2 (one per distinct user)", got)
	}

	// Per-session path re-reads for every session for comparison.
	cs.unitCalls.Store(0)
	cs.householdCalls.Store(0)
	for i := range streams {
		e.EvaluateSession(ctx, &streams[i], streams)
	}
	if got := cs.unitCalls.Load(); got != int32(len(streams)) {
		t.Errorf("per-session GetUnitSystem calls = %d, want %d", got, len(streams))
	}
}

// TestEngine_EvaluateSessions_MatchesPerSession verifies the batched path
// produces the same violations as evaluating each session individually.
func TestEngine_EvaluateSessions_MatchesPerSession(t *testing.T) {
	run := func(t *testing.T, batched bool) int {
		e, s := setupTestEngine(t)
		ctx := context.Background()

		config := models.ConcurrentStreamsConfig{MaxStreams: 1}
		configJSON, _ := json.Marshal(config)
		rule := &models.Rule{Name: "Max 1", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: configJSON}
		if err := s.CreateRule(rule); err != nil {
			t.Fatalf("CreateRule: %v", err)
		}
		e.RefreshRules()

		now := time.Now().UTC()
		streams := []models.ActiveStream{
			{SessionID: "a", UserName: "u1", IPAddress: "10.0.0.1", StartedAt: now},
			{SessionID: "b", UserName: "u1", IPAddress: "10.0.0.2", StartedAt: now.Add(time.Second)},
			{SessionID: "c", UserName: "u2", IPAddress: "10.0.0.3", StartedAt: now},
		}

		if batched {
			e.EvaluateSessions(ctx, streams)
		} else {
			for i := range streams {
				e.EvaluateSession(ctx, &streams[i], streams)
			}
		}

		result, err := s.ListViolations(1, 100, store.ViolationFilters{})
		if err != nil {
			t.Fatalf("ListViolations: %v", err)
		}
		return result.Total
	}

	perSession := run(t, false)
	batched := run(t, true)
	if perSession != batched {
		t.Errorf("violation count mismatch: per-session=%d batched=%d", perSession, batched)
	}
	if batched == 0 {
		t.Errorf("expected at least one violation, got 0")
	}
}

func setupTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	e := NewEngine(s, nil, DefaultEngineConfig())
	return e, s
}

type mockGeoResolver struct {
	results map[string]*models.GeoResult
}

func (m *mockGeoResolver) Lookup(ctx context.Context, ip string) (*models.GeoResult, error) {
	if result, ok := m.results[ip]; ok {
		return result, nil
	}
	return nil, nil
}

type mockNotifier struct {
	mu            sync.Mutex
	notifications []*models.RuleViolation
}

func (m *mockNotifier) Notify(ctx context.Context, v *models.RuleViolation, channels []models.NotificationChannel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, v)
	return nil
}

func (m *mockNotifier) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.notifications)
}

func (m *mockNotifier) lastActionTaken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.notifications) == 0 {
		return ""
	}
	return m.notifications[len(m.notifications)-1].ActionTaken
}

func TestEngine_RegisterEvaluator(t *testing.T) {
	e, _ := setupTestEngine(t)

	evaluators := e.GetEvaluators()
	if _, ok := evaluators[models.RuleTypeConcurrentStreams]; !ok {
		t.Error("expected ConcurrentStreams evaluator to be registered")
	}
	if _, ok := evaluators[models.RuleTypeGeoRestriction]; !ok {
		t.Error("expected GeoRestriction evaluator to be registered")
	}
	if _, ok := evaluators[models.RuleTypeSimultaneousLocs]; !ok {
		t.Error("expected SimultaneousLocs evaluator to be registered")
	}
}

func TestEngine_EvaluateSession_NoRules(t *testing.T) {
	e, _ := setupTestEngine(t)
	ctx := context.Background()

	stream := &models.ActiveStream{
		SessionID: "test",
		UserName:  "testuser",
		IPAddress: "192.168.1.1",
	}

	e.EvaluateSession(ctx, stream, []models.ActiveStream{*stream})
}

func TestEngine_EvaluateSession_ConcurrentStreams(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 2}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 2 Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now},
		{SessionID: "c", UserName: "testuser", IPAddress: "192.168.1.3", StartedAt: now},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, err := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if err != nil {
		t.Fatalf("ListViolations: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total violations = %d, want 1", result.Total)
	}
	if result.Total > 0 && result.Items[0].RuleID != rule.ID {
		t.Errorf("RuleID = %d, want %d", result.Items[0].RuleID, rule.ID)
	}
}

func TestEngine_EvaluateSession_ViolationCooldown(t *testing.T) {
	e, s := setupTestEngine(t)
	e.violationCooldown = 1 * time.Hour
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 1 Stream",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Fatalf("Expected 1 violation after first evaluation, got %d", result.Total)
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ = s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Errorf("Expected still 1 violation due to cooldown, got %d", result.Total)
	}
}

func TestEngine_EvaluateSession_TrustScoreDecrement(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 1 Stream",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	ts, _ := s.GetUserTrustScore("testuser")
	if ts.Score != 100 {
		t.Fatalf("Initial score = %d, want 100", ts.Score)
	}

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	ts, _ = s.GetUserTrustScore("testuser")
	if ts.Score >= 100 {
		t.Errorf("Trust score should have decreased, got %d", ts.Score)
	}
	if ts.ViolationCount != 1 {
		t.Errorf("ViolationCount = %d, want 1", ts.ViolationCount)
	}
}

func TestEngine_EvaluateSession_GeoRestriction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.2.3.4": {IP: "1.2.3.4", Country: "RU", City: "Moscow", Lat: 55.75, Lng: 37.62},
		},
	}

	e := NewEngine(s, geo, DefaultEngineConfig())
	ctx := context.Background()

	config := models.GeoRestrictionConfig{
		AllowedCountries: []string{"US", "CA", "GB"},
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "US/CA/GB Only",
		Type:    models.RuleTypeGeoRestriction,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	stream := &models.ActiveStream{
		SessionID: "test",
		UserName:  "testuser",
		IPAddress: "1.2.3.4",
	}

	e.EvaluateSession(ctx, stream, []models.ActiveStream{*stream})

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Errorf("Expected 1 geo violation, got %d", result.Total)
	}
	if result.Total > 0 && result.Items[0].Severity != models.SeverityCritical {
		t.Errorf("Severity = %s, want critical", result.Items[0].Severity)
	}
}

func TestEngine_EvaluateSession_DisabledRule(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Disabled Rule",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: false,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 0 {
		t.Errorf("Expected 0 violations for disabled rule, got %d", result.Total)
	}
}

func TestEngine_EvaluateSession_WithNotifier(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	notifier := &mockNotifier{}
	e.SetNotifier(notifier)

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 1 Stream",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)

	channel := &models.NotificationChannel{
		Name:        "Test Discord",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url":"https://discord.com/api/webhooks/test"}`),
		Enabled:     true,
	}
	s.CreateNotificationChannel(channel)
	s.LinkRuleToChannel(rule.ID, channel.ID)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	// Wait for notification goroutine to complete
	e.WaitForNotifications()

	if notifier.count() != 1 {
		t.Errorf("Expected 1 notification, got %d", notifier.count())
	}
}

func TestEngine_RefreshRules(t *testing.T) {
	e, s := setupTestEngine(t)

	rules, _ := e.getEnabledRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules initially, got %d", len(rules))
	}

	config := models.ConcurrentStreamsConfig{MaxStreams: 2}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "New Rule",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)

	if err := e.RefreshRules(); err != nil {
		t.Fatalf("RefreshRules: %v", err)
	}

	rules, _ = e.getEnabledRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule after refresh, got %d", len(rules))
	}
}

type mockMediaServer struct {
	id            int64
	serverType    models.ServerType
	terminatedIDs []string
	terminateErr  error
	mu            sync.Mutex
}

func (m *mockMediaServer) Name() string                             { return "test-server" }
func (m *mockMediaServer) Type() models.ServerType                  { return m.serverType }
func (m *mockMediaServer) ServerID() int64                          { return m.id }
func (m *mockMediaServer) TestConnection(ctx context.Context) error { return nil }
func (m *mockMediaServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockMediaServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockMediaServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	return nil, nil
}
func (m *mockMediaServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockMediaServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockMediaServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockMediaServer) DeleteItem(ctx context.Context, itemID string) error { return nil }
func (m *mockMediaServer) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	return nil, nil
}
func (m *mockMediaServer) GetEpisodes(ctx context.Context, seasonID string) ([]models.Episode, error) {
	return nil, nil
}
func (m *mockMediaServer) TerminateSession(ctx context.Context, sessionID string, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminatedIDs = append(m.terminatedIDs, sessionID)
	return m.terminateErr
}
func (m *mockMediaServer) getTerminatedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.terminatedIDs...)
}

type mockServerResolver struct {
	servers map[int64]media.MediaServer
}

func (r *mockServerResolver) GetServer(id int64) (media.MediaServer, bool) {
	s, ok := r.servers[id]
	return s, ok
}

func TestEngine_ExemptUser_SkipsEvaluation(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 1 Stream",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)

	// Exempt "testuser" from this rule
	if err := s.SetRuleExemptions(rule.ID, []string{"testuser"}); err != nil {
		t.Fatalf("SetRuleExemptions: %v", err)
	}
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second)},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 0 {
		t.Errorf("Expected 0 violations for exempt user, got %d", result.Total)
	}
}

func TestEngine_ExemptUser_CaseInsensitive(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Max 1 Stream",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)

	// Exempt with different case
	if err := s.SetRuleExemptions(rule.ID, []string{"TestUser"}); err != nil {
		t.Fatalf("SetRuleExemptions: %v", err)
	}
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second)},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 0 {
		t.Errorf("Expected 0 violations for case-insensitive exempt user, got %d", result.Total)
	}
}

func TestEngine_AutoTerminate_ConcurrentStreams(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	ms := &mockMediaServer{id: 1, serverType: models.ServerTypeEmby}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{1: ms}}
	e.SetServerResolver(resolver)

	config := models.ConcurrentStreamsConfig{
		MaxStreams:    1,
		AutoTerminate: true,
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Auto-Kill Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "old", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "new", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second)},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	terminated := ms.getTerminatedIDs()
	if len(terminated) != 1 {
		t.Fatalf("Expected 1 terminated session, got %d", len(terminated))
	}
	if terminated[0] != "new" {
		t.Errorf("Expected newest session 'new' to be terminated, got %q", terminated[0])
	}

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Fatalf("Expected 1 violation, got %d", result.Total)
	}
	if result.Items[0].ActionTaken != "terminated" {
		t.Errorf("ActionTaken = %q, want %q", result.Items[0].ActionTaken, "terminated")
	}
}

func TestEngine_AutoTerminate_ConcurrentStreams_CountPausedAsOne(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	ms := &mockMediaServer{id: 1, serverType: models.ServerTypeEmby}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{1: ms}}
	e.SetServerResolver(resolver)

	config := models.ConcurrentStreamsConfig{
		MaxStreams:       2,
		CountPausedAsOne: true,
		AutoTerminate:    true,
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Auto-Kill Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "active", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now, State: models.SessionStatePlaying},
		{SessionID: "paused1", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second), State: models.SessionStatePaused},
		{SessionID: "paused2", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.3", StartedAt: now.Add(2 * time.Second), State: models.SessionStatePaused},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	terminated := ms.getTerminatedIDs()
	if len(terminated) != 0 {
		t.Fatalf("Expected no terminated sessions (1 active + 2 paused collapsed = 2, at max), got %v", terminated)
	}

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 0 {
		t.Fatalf("Expected 0 violations, got %d", result.Total)
	}
}

func TestEngine_AutoTerminate_Failed(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	ms := &mockMediaServer{
		id:           1,
		serverType:   models.ServerTypeEmby,
		terminateErr: fmt.Errorf("server unreachable"),
	}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{1: ms}}
	e.SetServerResolver(resolver)

	config := models.ConcurrentStreamsConfig{
		MaxStreams:    1,
		AutoTerminate: true,
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Auto-Kill Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "old", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "new", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second)},
	}

	e.EvaluateSession(ctx, &streams[0], streams)

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Fatalf("Expected 1 violation, got %d", result.Total)
	}
	if result.Items[0].ActionTaken != "terminate_failed" {
		t.Errorf("ActionTaken = %q, want %q", result.Items[0].ActionTaken, "terminate_failed")
	}
}

func TestEngine_AutoTerminate_NotificationGetsActionTaken(t *testing.T) {
	e, s := setupTestEngine(t)
	ctx := context.Background()

	ms := &mockMediaServer{id: 1, serverType: models.ServerTypeEmby}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{1: ms}}
	e.SetServerResolver(resolver)

	notif := &mockNotifier{}
	e.SetNotifier(notif)

	config := models.ConcurrentStreamsConfig{
		MaxStreams:    1,
		AutoTerminate: true,
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "Auto-Kill Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)

	channel := &models.NotificationChannel{
		Name:        "Test",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url":"https://discord.com/api/webhooks/test"}`),
		Enabled:     true,
	}
	s.CreateNotificationChannel(channel)
	s.LinkRuleToChannel(rule.ID, channel.ID)
	e.RefreshRules()

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "old", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.1", StartedAt: now},
		{SessionID: "new", ServerID: 1, UserName: "testuser", IPAddress: "192.168.1.2", StartedAt: now.Add(time.Second)},
	}

	e.EvaluateSession(ctx, &streams[0], streams)
	e.WaitForNotifications()

	if notif.count() != 1 {
		t.Fatalf("Expected 1 notification, got %d", notif.count())
	}
	if got := notif.lastActionTaken(); got != "terminated" {
		t.Errorf("Notification violation ActionTaken = %q, want %q", got, "terminated")
	}
}

func TestEngine_AutoTerminate_GeoRestriction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.2.3.4": {IP: "1.2.3.4", Country: "RU", City: "Moscow", Lat: 55.75, Lng: 37.62},
		},
	}

	e := NewEngine(s, geo, DefaultEngineConfig())
	ctx := context.Background()

	ms := &mockMediaServer{id: 1, serverType: models.ServerTypeEmby}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{1: ms}}
	e.SetServerResolver(resolver)

	config := models.GeoRestrictionConfig{
		AllowedCountries: []string{"US"},
		AutoTerminate:    true,
		TerminateMessage: "Blocked region",
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		Name:    "US Only",
		Type:    models.RuleTypeGeoRestriction,
		Enabled: true,
		Config:  configJSON,
	}
	s.CreateRule(rule)
	e.RefreshRules()

	stream := &models.ActiveStream{
		SessionID: "sess-1",
		ServerID:  1,
		UserName:  "testuser",
		IPAddress: "1.2.3.4",
	}

	e.EvaluateSession(ctx, stream, []models.ActiveStream{*stream})

	terminated := ms.getTerminatedIDs()
	if len(terminated) != 1 {
		t.Fatalf("Expected 1 terminated session, got %d", len(terminated))
	}
	if terminated[0] != "sess-1" {
		t.Errorf("Expected session 'sess-1' to be terminated, got %q", terminated[0])
	}

	result, _ := s.ListViolations(1, 10, store.ViolationFilters{UserName: "testuser"})
	if result.Total != 1 {
		t.Fatalf("Expected 1 violation, got %d", result.Total)
	}
	if result.Items[0].ActionTaken != "terminated" {
		t.Errorf("ActionTaken = %q, want %q", result.Items[0].ActionTaken, "terminated")
	}
}
