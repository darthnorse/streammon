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

type mockHistoryQuerierForISPVelocity struct {
	isps []string
}

func (m *mockHistoryQuerierForISPVelocity) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForISPVelocity) GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForISPVelocity) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	return false, nil
}

func (m *mockHistoryQuerierForISPVelocity) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForISPVelocity) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	return nil, nil
}

func (m *mockHistoryQuerierForISPVelocity) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	return m.isps, nil
}

type mockGeoResolverForISP struct {
	isp string
}

func (m *mockGeoResolverForISP) Lookup(ctx context.Context, ip string) (*models.GeoResult, error) {
	return &models.GeoResult{
		ISP:     m.isp,
		City:    "Test City",
		Country: "US",
		Lat:     40.0,
		Lng:     -74.0,
	}, nil
}

func TestISPVelocityEvaluator_TooManyISPs(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T"},
	}
	geo := &mockGeoResolverForISP{isp: "T-Mobile"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: "T-Mobile"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Contains(t, result.Violation.Message, "ISPs")
	assert.Equal(t, models.SeverityInfo, result.Violation.Severity)
}

func TestISPVelocityEvaluator_WithinLimit(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon"},
	}
	geo := &mockGeoResolverForISP{isp: "AT&T"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: "AT&T"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestISPVelocityEvaluator_SameISPNotCounted(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T"},
	}
	geo := &mockGeoResolverForISP{isp: "Comcast"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: "Comcast"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestISPVelocityEvaluator_NoISP(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T"},
	}

	evaluator := NewISPVelocityEvaluator(nil, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: ""},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestISPVelocityEvaluator_NilStream(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{}
	evaluator := NewISPVelocityEvaluator(nil, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: nil,
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestISPVelocityEvaluator_Type(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{}
	evaluator := NewISPVelocityEvaluator(nil, mock)

	assert.Equal(t, models.RuleTypeISPVelocity, evaluator.Type())
}

func TestISPVelocityEvaluator_SeverityEscalation(t *testing.T) {
	// 6 ISPs when max is 3 should be critical (2x the limit)
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T", "T-Mobile", "Sprint"},
	}
	geo := &mockGeoResolverForISP{isp: "Cox"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: "Cox"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Equal(t, models.SeverityCritical, result.Violation.Severity)
}

func TestISPVelocityEvaluator_UsesGeoResolverWhenNoGeoData(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T"},
	}
	geo := &mockGeoResolverForISP{isp: "T-Mobile"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: nil, // No geo data provided
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
}

func TestISPVelocityEvaluator_ZeroStartedAtFallback(t *testing.T) {
	mock := &mockHistoryQuerierForISPVelocity{
		isps: []string{"Comcast", "Verizon", "AT&T"},
	}
	geo := &mockGeoResolverForISP{isp: "T-Mobile"}

	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Time{}, // Zero time
		},
		GeoData: &models.GeoResult{ISP: "T-Mobile"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
}

type mockHistoryQuerierWithISPError struct {
	mockHistoryQuerierForISPVelocity
	err error
}

func (m *mockHistoryQuerierWithISPError) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	return nil, m.err
}

func TestISPVelocityEvaluator_StoreError(t *testing.T) {
	mock := &mockHistoryQuerierWithISPError{err: assert.AnError}
	geo := &mockGeoResolverForISP{isp: "Comcast"}
	evaluator := NewISPVelocityEvaluator(geo, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "ISP Velocity",
		Type:   models.RuleTypeISPVelocity,
		Config: json.RawMessage(`{"max_isps": 3, "time_window_hours": 168}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.2.3.4",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{ISP: "Comcast"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting recent ISPs")
	assert.Nil(t, result)
}
