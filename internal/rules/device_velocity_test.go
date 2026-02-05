package rules

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"streammon/internal/models"
)

type mockHistoryQuerierForDeviceVelocity struct {
	deviceStreams []*models.WatchHistoryEntry
}

func (m *mockHistoryQuerierForDeviceVelocity) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForDeviceVelocity) GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForDeviceVelocity) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	return false, nil
}

func (m *mockHistoryQuerierForDeviceVelocity) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForDeviceVelocity) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	var devices []models.DeviceInfo
	seen := make(map[string]bool)
	cutoff := beforeTime.Add(-time.Duration(withinHours) * time.Hour)

	for _, s := range m.deviceStreams {
		if s.StartedAt.Before(cutoff) || s.StartedAt.After(beforeTime) {
			continue
		}
		key := s.Player + "|" + s.Platform
		if !seen[key] {
			seen[key] = true
			devices = append(devices, models.DeviceInfo{Player: s.Player, Platform: s.Platform})
		}
	}
	return devices, nil
}

func (m *mockHistoryQuerierForDeviceVelocity) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	return nil, nil
}

func TestDeviceVelocityEvaluator_TooManyDevices(t *testing.T) {
	now := time.Now().UTC()

	mock := &mockHistoryQuerierForDeviceVelocity{
		deviceStreams: []*models.WatchHistoryEntry{
			{Player: "Plex Web", Platform: "Chrome", StartedAt: now.Add(-30 * time.Minute)},
			{Player: "Plex for iOS", Platform: "iOS", StartedAt: now.Add(-20 * time.Minute)},
			{Player: "Plex for Android", Platform: "Android", StartedAt: now.Add(-10 * time.Minute)},
		},
	}

	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 2, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			Player:    "Plex for Roku",
			Platform:  "Roku",
			StartedAt: now,
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Contains(t, result.Violation.Message, "devices")
}

func TestDeviceVelocityEvaluator_WithinLimit(t *testing.T) {
	now := time.Now().UTC()

	mock := &mockHistoryQuerierForDeviceVelocity{
		deviceStreams: []*models.WatchHistoryEntry{
			{Player: "Plex Web", Platform: "Chrome", StartedAt: now.Add(-30 * time.Minute)},
		},
	}

	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 3, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			Player:    "Plex for iOS",
			Platform:  "iOS",
			StartedAt: now,
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDeviceVelocityEvaluator_NoHistory(t *testing.T) {
	mock := &mockHistoryQuerierForDeviceVelocity{
		deviceStreams: nil,
	}

	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 2, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			Player:    "Plex Web",
			Platform:  "Chrome",
			StartedAt: time.Now().UTC(),
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDeviceVelocityEvaluator_SameDeviceNotCounted(t *testing.T) {
	now := time.Now().UTC()

	mock := &mockHistoryQuerierForDeviceVelocity{
		deviceStreams: []*models.WatchHistoryEntry{
			{Player: "Plex Web", Platform: "Chrome", StartedAt: now.Add(-30 * time.Minute)},
			{Player: "Plex Web", Platform: "Chrome", StartedAt: now.Add(-20 * time.Minute)},
		},
	}

	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 2, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			Player:    "Plex Web",
			Platform:  "Chrome",
			StartedAt: now,
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDeviceVelocityEvaluator_NilStream(t *testing.T) {
	mock := &mockHistoryQuerierForDeviceVelocity{}
	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 2, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: nil,
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestDeviceVelocityEvaluator_Type(t *testing.T) {
	mock := &mockHistoryQuerierForDeviceVelocity{}
	evaluator := NewDeviceVelocityEvaluator(mock)

	assert.Equal(t, models.RuleTypeDeviceVelocity, evaluator.Type())
}

type mockHistoryQuerierWithDeviceError struct {
	mockHistoryQuerierForDeviceVelocity
	err error
}

func (m *mockHistoryQuerierWithDeviceError) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	return nil, m.err
}

func TestDeviceVelocityEvaluator_StoreError(t *testing.T) {
	mock := &mockHistoryQuerierWithDeviceError{err: assert.AnError}
	evaluator := NewDeviceVelocityEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Device Velocity",
		Type:   models.RuleTypeDeviceVelocity,
		Config: json.RawMessage(`{"max_devices_per_hour": 2, "time_window_hours": 1}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			Player:    "Plex Web",
			Platform:  "Chrome",
			StartedAt: time.Now().UTC(),
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting recent devices")
	assert.Nil(t, result)
}
