package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"streammon/internal/models"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRuleCRUD(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{
		Name:    "Test Concurrent Streams",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  json.RawMessage(`{"max_streams": 3}`),
	}

	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if rule.ID == 0 {
		t.Error("expected rule ID to be set")
	}

	got, err := s.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.Name != rule.Name {
		t.Errorf("Name = %q, want %q", got.Name, rule.Name)
	}
	if got.Type != rule.Type {
		t.Errorf("Type = %q, want %q", got.Type, rule.Type)
	}
	if !got.Enabled {
		t.Error("expected Enabled = true")
	}

	got.Name = "Updated Rule"
	got.Enabled = false
	if err := s.UpdateRule(got); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}

	got, err = s.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule after update: %v", err)
	}
	if got.Name != "Updated Rule" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Rule")
	}
	if got.Enabled {
		t.Error("expected Enabled = false")
	}

	if err := s.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}

	_, err = s.GetRule(rule.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestListRules(t *testing.T) {
	s := setupTestStore(t)

	rules := []models.Rule{
		{Name: "Rule A", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)},
		{Name: "Rule B", Type: models.RuleTypeGeoRestriction, Enabled: false, Config: json.RawMessage(`{}`)},
		{Name: "Rule C", Type: models.RuleTypeImpossibleTravel, Enabled: true, Config: json.RawMessage(`{}`)},
	}
	for i := range rules {
		if err := s.CreateRule(&rules[i]); err != nil {
			t.Fatalf("CreateRule %d: %v", i, err)
		}
	}

	all, err := s.ListRules()
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListRules got %d, want 3", len(all))
	}

	enabled, err := s.ListEnabledRules()
	if err != nil {
		t.Fatalf("ListEnabledRules: %v", err)
	}
	if len(enabled) != 2 {
		t.Errorf("ListEnabledRules got %d, want 2", len(enabled))
	}

	byType, err := s.ListRulesByType(models.RuleTypeConcurrentStreams)
	if err != nil {
		t.Fatalf("ListRulesByType: %v", err)
	}
	if len(byType) != 1 {
		t.Errorf("ListRulesByType got %d, want 1", len(byType))
	}
}

func TestViolationCRUD(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{
		Name:    "Test Rule",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  json.RawMessage(`{}`),
	}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	now := time.Now().UTC()
	violation := &models.RuleViolation{
		RuleID:          rule.ID,
		UserName:        "testuser",
		Severity:        models.SeverityWarning,
		Message:         "3 concurrent streams detected",
		Details:         map[string]interface{}{"stream_count": 3},
		ConfidenceScore: 95.0,
		OccurredAt:      now,
	}

	if err := s.InsertViolation(violation); err != nil {
		t.Fatalf("InsertViolation: %v", err)
	}
	if violation.ID == 0 {
		t.Error("expected violation ID to be set")
	}

	result, err := s.ListViolations(1, 10, ViolationFilters{})
	if err != nil {
		t.Fatalf("ListViolations: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
	if len(result.Items) != 1 {
		t.Errorf("Items = %d, want 1", len(result.Items))
	}
	if result.Items[0].RuleName != rule.Name {
		t.Errorf("RuleName = %q, want %q", result.Items[0].RuleName, rule.Name)
	}

	result, err = s.ListViolations(1, 10, ViolationFilters{UserName: "testuser"})
	if err != nil {
		t.Fatalf("ListViolations with filter: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}

	result, err = s.ListViolations(1, 10, ViolationFilters{UserName: "otheruser"})
	if err != nil {
		t.Fatalf("ListViolations with different user: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
}

func TestViolationFilters(t *testing.T) {
	s := setupTestStore(t)

	rule1 := &models.Rule{Name: "Rule1", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)}
	rule2 := &models.Rule{Name: "Rule2", Type: models.RuleTypeGeoRestriction, Enabled: true, Config: json.RawMessage(`{}`)}
	s.CreateRule(rule1)
	s.CreateRule(rule2)

	now := time.Now().UTC()
	violations := []models.RuleViolation{
		{RuleID: rule1.ID, UserName: "user1", Severity: models.SeverityWarning, Message: "v1", ConfidenceScore: 80, OccurredAt: now.Add(-2 * time.Hour)},
		{RuleID: rule1.ID, UserName: "user2", Severity: models.SeverityCritical, Message: "v2", ConfidenceScore: 95, OccurredAt: now.Add(-1 * time.Hour)},
		{RuleID: rule2.ID, UserName: "user1", Severity: models.SeverityInfo, Message: "v3", ConfidenceScore: 60, OccurredAt: now},
	}
	for i := range violations {
		s.InsertViolation(&violations[i])
	}

	tests := []struct {
		name    string
		filters ViolationFilters
		want    int
	}{
		{"no filter", ViolationFilters{}, 3},
		{"by user", ViolationFilters{UserName: "user1"}, 2},
		{"by rule", ViolationFilters{RuleID: rule1.ID}, 2},
		{"by severity", ViolationFilters{Severity: models.SeverityCritical}, 1},
		{"by min confidence", ViolationFilters{MinConfidence: 90}, 1},
		{"by since", ViolationFilters{Since: now.Add(-90 * time.Minute)}, 2},
		{"combined", ViolationFilters{UserName: "user1", RuleID: rule1.ID}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.ListViolations(1, 10, tt.filters)
			if err != nil {
				t.Fatalf("ListViolations: %v", err)
			}
			if result.Total != tt.want {
				t.Errorf("Total = %d, want %d", result.Total, tt.want)
			}
		})
	}
}

func TestHouseholdLocations(t *testing.T) {
	s := setupTestStore(t)

	now := time.Now().UTC()
	loc := &models.HouseholdLocation{
		UserName:     "testuser",
		IPAddress:    "192.168.1.100",
		City:         "New York",
		Country:      "US",
		Latitude:     40.7128,
		Longitude:    -74.0060,
		AutoLearned:  true,
		Trusted:      true,
		SessionCount: 1,
		FirstSeen:    now,
		LastSeen:     now,
	}

	if err := s.UpsertHouseholdLocation(loc); err != nil {
		t.Fatalf("UpsertHouseholdLocation: %v", err)
	}

	locations, err := s.ListHouseholdLocations("testuser")
	if err != nil {
		t.Fatalf("ListHouseholdLocations: %v", err)
	}
	if len(locations) != 1 {
		t.Fatalf("got %d locations, want 1", len(locations))
	}
	if locations[0].City != "New York" {
		t.Errorf("City = %q, want %q", locations[0].City, "New York")
	}
	if !locations[0].Trusted {
		t.Error("expected Trusted = true")
	}

	loc.LastSeen = now.Add(time.Hour)
	if err := s.UpsertHouseholdLocation(loc); err != nil {
		t.Fatalf("UpsertHouseholdLocation again: %v", err)
	}

	locations, err = s.ListHouseholdLocations("testuser")
	if err != nil {
		t.Fatalf("ListHouseholdLocations after upsert: %v", err)
	}
	if len(locations) != 1 {
		t.Errorf("got %d locations after upsert, want 1", len(locations))
	}
	if locations[0].SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", locations[0].SessionCount)
	}

	if err := s.UpdateHouseholdTrusted(locations[0].ID, false); err != nil {
		t.Fatalf("UpdateHouseholdTrusted: %v", err)
	}

	trusted, err := s.ListTrustedHouseholdLocations("testuser")
	if err != nil {
		t.Fatalf("ListTrustedHouseholdLocations: %v", err)
	}
	if len(trusted) != 0 {
		t.Errorf("got %d trusted locations, want 0", len(trusted))
	}
}

func TestUserTrustScore(t *testing.T) {
	s := setupTestStore(t)

	ts, err := s.GetUserTrustScore("newuser")
	if err != nil {
		t.Fatalf("GetUserTrustScore: %v", err)
	}
	if ts.Score != 100 {
		t.Errorf("Score = %d, want 100 for new user", ts.Score)
	}

	now := time.Now().UTC()
	if err := s.DecrementTrustScore("newuser", 10, now); err != nil {
		t.Fatalf("DecrementTrustScore: %v", err)
	}

	ts, err = s.GetUserTrustScore("newuser")
	if err != nil {
		t.Fatalf("GetUserTrustScore after decrement: %v", err)
	}
	if ts.Score != 90 {
		t.Errorf("Score = %d, want 90", ts.Score)
	}
	if ts.ViolationCount != 1 {
		t.Errorf("ViolationCount = %d, want 1", ts.ViolationCount)
	}

	if err := s.DecrementTrustScore("newuser", 5, now); err != nil {
		t.Fatalf("DecrementTrustScore again: %v", err)
	}

	ts, err = s.GetUserTrustScore("newuser")
	if err != nil {
		t.Fatalf("GetUserTrustScore after second decrement: %v", err)
	}
	if ts.Score != 85 {
		t.Errorf("Score = %d, want 85", ts.Score)
	}
	if ts.ViolationCount != 2 {
		t.Errorf("ViolationCount = %d, want 2", ts.ViolationCount)
	}
}

func TestNotificationChannelCRUD(t *testing.T) {
	s := setupTestStore(t)

	channel := &models.NotificationChannel{
		Name:        "Discord Alerts",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url": "https://discord.com/api/webhooks/123/abc"}`),
		Enabled:     true,
	}

	if err := s.CreateNotificationChannel(channel); err != nil {
		t.Fatalf("CreateNotificationChannel: %v", err)
	}
	if channel.ID == 0 {
		t.Error("expected channel ID to be set")
	}

	got, err := s.GetNotificationChannel(channel.ID)
	if err != nil {
		t.Fatalf("GetNotificationChannel: %v", err)
	}
	if got.Name != channel.Name {
		t.Errorf("Name = %q, want %q", got.Name, channel.Name)
	}

	got.Enabled = false
	if err := s.UpdateNotificationChannel(got); err != nil {
		t.Fatalf("UpdateNotificationChannel: %v", err)
	}

	enabled, err := s.ListEnabledNotificationChannels()
	if err != nil {
		t.Fatalf("ListEnabledNotificationChannels: %v", err)
	}
	if len(enabled) != 0 {
		t.Errorf("got %d enabled channels, want 0", len(enabled))
	}

	if err := s.DeleteNotificationChannel(channel.ID); err != nil {
		t.Fatalf("DeleteNotificationChannel: %v", err)
	}

	_, err = s.GetNotificationChannel(channel.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestRuleChannelLinking(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{Name: "Test Rule", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)}
	s.CreateRule(rule)

	channel1 := &models.NotificationChannel{Name: "Discord", ChannelType: models.ChannelTypeDiscord, Config: json.RawMessage(`{"webhook_url":"url1"}`), Enabled: true}
	channel2 := &models.NotificationChannel{Name: "Webhook", ChannelType: models.ChannelTypeWebhook, Config: json.RawMessage(`{"url":"url2"}`), Enabled: true}
	channel3 := &models.NotificationChannel{Name: "Disabled", ChannelType: models.ChannelTypePushover, Config: json.RawMessage(`{"user_key":"x","api_token":"y"}`), Enabled: false}
	s.CreateNotificationChannel(channel1)
	s.CreateNotificationChannel(channel2)
	s.CreateNotificationChannel(channel3)

	s.LinkRuleToChannel(rule.ID, channel1.ID)
	s.LinkRuleToChannel(rule.ID, channel2.ID)
	s.LinkRuleToChannel(rule.ID, channel3.ID)

	channels, err := s.GetChannelsForRule(rule.ID)
	if err != nil {
		t.Fatalf("GetChannelsForRule: %v", err)
	}
	if len(channels) != 2 {
		t.Errorf("got %d channels, want 2 (disabled excluded)", len(channels))
	}

	s.UnlinkRuleFromChannel(rule.ID, channel1.ID)

	channels, err = s.GetChannelsForRule(rule.ID)
	if err != nil {
		t.Fatalf("GetChannelsForRule after unlink: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("got %d channels after unlink, want 1", len(channels))
	}
}

func TestViolationExistsRecent(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{Name: "Test", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)}
	s.CreateRule(rule)

	// Test time-based deduplication (no session key)
	exists, err := s.ViolationExistsRecent(rule.ID, "testuser", "", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent: %v", err)
	}
	if exists {
		t.Error("expected no recent violation")
	}

	v := &models.RuleViolation{
		RuleID:          rule.ID,
		UserName:        "testuser",
		Severity:        models.SeverityWarning,
		Message:         "test",
		ConfidenceScore: 80,
		OccurredAt:      time.Now().UTC(),
	}
	s.InsertViolation(v)

	exists, err = s.ViolationExistsRecent(rule.ID, "testuser", "", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent after insert: %v", err)
	}
	if !exists {
		t.Error("expected recent violation to exist")
	}

	exists, err = s.ViolationExistsRecent(rule.ID, "otheruser", "", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent for other user: %v", err)
	}
	if exists {
		t.Error("expected no recent violation for other user")
	}
}

func TestViolationExistsRecentWithSessionKey(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{Name: "Test", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)}
	s.CreateRule(rule)

	// Test session-based deduplication
	exists, err := s.ViolationExistsRecent(rule.ID, "testuser", "session123", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent: %v", err)
	}
	if exists {
		t.Error("expected no violation for session")
	}

	v := &models.RuleViolation{
		RuleID:          rule.ID,
		UserName:        "testuser",
		Severity:        models.SeverityWarning,
		Message:         "test",
		ConfidenceScore: 80,
		SessionKey:      "session123",
		OccurredAt:      time.Now().UTC(),
	}
	s.InsertViolation(v)

	// Same session should find existing violation
	exists, err = s.ViolationExistsRecent(rule.ID, "testuser", "session123", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent after insert: %v", err)
	}
	if !exists {
		t.Error("expected violation to exist for same session")
	}

	// Different session should not find violation
	exists, err = s.ViolationExistsRecent(rule.ID, "testuser", "session456", time.Hour)
	if err != nil {
		t.Fatalf("ViolationExistsRecent for different session: %v", err)
	}
	if exists {
		t.Error("expected no violation for different session")
	}
}

func TestInsertViolationWithTx(t *testing.T) {
	s := setupTestStore(t)

	rule := &models.Rule{Name: "Test", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{}`)}
	s.CreateRule(rule)

	v := &models.RuleViolation{
		RuleID:          rule.ID,
		UserName:        "testuser",
		Severity:        models.SeverityWarning,
		Message:         "test violation",
		ConfidenceScore: 90,
		OccurredAt:      time.Now().UTC(),
	}

	ctx := context.Background()
	if err := s.InsertViolationWithTx(ctx, v, 15); err != nil {
		t.Fatalf("InsertViolationWithTx: %v", err)
	}
	if v.ID == 0 {
		t.Error("expected violation ID to be set")
	}

	ts, err := s.GetUserTrustScore("testuser")
	if err != nil {
		t.Fatalf("GetUserTrustScore: %v", err)
	}
	if ts.Score != 85 {
		t.Errorf("Score = %d, want 85", ts.Score)
	}
	if ts.ViolationCount != 1 {
		t.Errorf("ViolationCount = %d, want 1", ts.ViolationCount)
	}
}

func TestAutoLearnHouseholdLocation(t *testing.T) {
	s := setupTestStore(t)

	now := time.Now().UTC()
	serverID := seedTestServer(t, s)

	// Seed geo cache for the test IP
	_, err := s.db.Exec(`INSERT INTO ip_geo_cache (ip, lat, lng, city, country, isp, cached_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"1.1.1.1", 40.7128, -74.0060, "New York", "US", "Comcast", now)
	if err != nil {
		t.Fatalf("seed geo cache: %v", err)
	}

	// Test: Not enough sessions - should not create household location
	// Seed only 5 sessions
	for i := 0; i < 5; i++ {
		entry := &models.WatchHistoryEntry{
			ServerID:  serverID,
			UserName:  "alice",
			Title:     "Movie",
			StartedAt: now.Add(-time.Duration(i) * time.Hour),
			IPAddress: "1.1.1.1",
			Player:    "Plex Web",
			Platform:  "Chrome",
			MediaType: models.MediaTypeMovie,
		}
		if err := s.InsertHistory(entry); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	created, err := s.AutoLearnHouseholdLocation("alice", "1.1.1.1", 10)
	if err != nil {
		t.Fatalf("AutoLearnHouseholdLocation (not enough sessions): %v", err)
	}
	if created {
		t.Error("expected created=false when below session threshold")
	}

	locations, err := s.ListHouseholdLocations("alice")
	if err != nil {
		t.Fatalf("ListHouseholdLocations: %v", err)
	}
	if len(locations) != 0 {
		t.Errorf("expected 0 household locations, got %d", len(locations))
	}

	// Add more sessions to reach threshold
	for i := 5; i < 12; i++ {
		entry := &models.WatchHistoryEntry{
			ServerID:  serverID,
			UserName:  "alice",
			Title:     "Movie",
			StartedAt: now.Add(-time.Duration(i) * time.Hour),
			IPAddress: "1.1.1.1",
			Player:    "Plex Web",
			Platform:  "Chrome",
			MediaType: models.MediaTypeMovie,
		}
		if err := s.InsertHistory(entry); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	// Test: Enough sessions - should create household location
	created, err = s.AutoLearnHouseholdLocation("alice", "1.1.1.1", 10)
	if err != nil {
		t.Fatalf("AutoLearnHouseholdLocation (enough sessions): %v", err)
	}
	if !created {
		t.Error("expected created=true when above session threshold")
	}

	locations, err = s.ListHouseholdLocations("alice")
	if err != nil {
		t.Fatalf("ListHouseholdLocations after auto-learn: %v", err)
	}
	if len(locations) != 1 {
		t.Fatalf("expected 1 household location, got %d", len(locations))
	}
	if !locations[0].AutoLearned {
		t.Error("expected AutoLearned=true")
	}
	if locations[0].Trusted {
		t.Error("expected Trusted=false for auto-learned location")
	}
	if locations[0].City != "New York" {
		t.Errorf("City = %q, want %q", locations[0].City, "New York")
	}

	// Test: Calling again should update session count, not create new
	created, err = s.AutoLearnHouseholdLocation("alice", "1.1.1.1", 10)
	if err != nil {
		t.Fatalf("AutoLearnHouseholdLocation (update): %v", err)
	}
	if created {
		t.Error("expected created=false when location already exists")
	}

	locations, err = s.ListHouseholdLocations("alice")
	if err != nil {
		t.Fatalf("ListHouseholdLocations after update: %v", err)
	}
	if len(locations) != 1 {
		t.Errorf("expected still 1 household location, got %d", len(locations))
	}

	// Test: Empty IP should be a no-op
	created, err = s.AutoLearnHouseholdLocation("alice", "", 10)
	if err != nil {
		t.Fatalf("AutoLearnHouseholdLocation (empty IP): %v", err)
	}
	if created {
		t.Error("expected created=false for empty IP")
	}
}

func seedTestServer(t *testing.T, s *Store) int64 {
	t.Helper()
	srv := &models.Server{
		Name:    "Test Server",
		Type:    models.ServerTypePlex,
		URL:     "http://localhost:32400",
		APIKey:  "test-key",
		Enabled: true,
	}
	if err := s.CreateServer(srv); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	return srv.ID
}
