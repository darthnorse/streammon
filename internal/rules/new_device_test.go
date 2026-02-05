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

type mockHistoryQuerier struct {
	hasDeviceBeenUsed bool
}

func (m *mockHistoryQuerier) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerier) GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerier) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	return m.hasDeviceBeenUsed, nil
}

func (m *mockHistoryQuerier) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockHistoryQuerier) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	return nil, nil
}

func (m *mockHistoryQuerier) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	return nil, nil
}

func TestNewDeviceEvaluator_NewDevice(t *testing.T) {
	mock := &mockHistoryQuerier{hasDeviceBeenUsed: false}
	evaluator := NewNewDeviceEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Device Alert",
		Type:   models.RuleTypeNewDevice,
		Config: json.RawMessage(`{"notify_on_new": true}`),
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
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Equal(t, models.SeverityInfo, result.Violation.Severity)
	assert.Contains(t, result.Violation.Message, "new device")
}

func TestNewDeviceEvaluator_ExistingDevice(t *testing.T) {
	mock := &mockHistoryQuerier{hasDeviceBeenUsed: true}
	evaluator := NewNewDeviceEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Device Alert",
		Type:   models.RuleTypeNewDevice,
		Config: json.RawMessage(`{"notify_on_new": true}`),
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

func TestNewDeviceEvaluator_Disabled(t *testing.T) {
	mock := &mockHistoryQuerier{hasDeviceBeenUsed: false}
	evaluator := NewNewDeviceEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Device Alert",
		Type:   models.RuleTypeNewDevice,
		Config: json.RawMessage(`{"notify_on_new": false}`),
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

type mockHistoryQuerierWithError struct {
	mockHistoryQuerier
	err error
}

func (m *mockHistoryQuerierWithError) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	return false, m.err
}

func TestNewDeviceEvaluator_StoreError(t *testing.T) {
	mock := &mockHistoryQuerierWithError{err: assert.AnError}
	evaluator := NewNewDeviceEvaluator(mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Device Alert",
		Type:   models.RuleTypeNewDevice,
		Config: json.RawMessage(`{"notify_on_new": true}`),
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
	assert.Contains(t, err.Error(), "checking device history")
	assert.Nil(t, result)
}
