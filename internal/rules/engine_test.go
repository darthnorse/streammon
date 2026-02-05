package rules

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

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
