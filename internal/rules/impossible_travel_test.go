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

type mockHistoryQuerierWithLastStream struct {
	mockHistoryQuerier
	lastStream *models.WatchHistoryEntry
}

func (m *mockHistoryQuerierWithLastStream) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return m.lastStream, nil
}

func TestImpossibleTravelEvaluator_ImpossibleTravel(t *testing.T) {
	now := time.Now().UTC()

	// NYC 1 hour ago
	mock := &mockHistoryQuerierWithLastStream{
		lastStream: &models.WatchHistoryEntry{
			UserName:  "alice",
			IPAddress: "1.1.1.1",
			StartedAt: now.Add(-1 * time.Hour),
		},
	}

	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"}, // ~5500 km from NYC
		},
	}

	evaluator := NewImpossibleTravelEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Impossible Travel",
		Type:   models.RuleTypeImpossibleTravel,
		Config: json.RawMessage(`{"max_speed_km_h": 800, "min_distance_km": 100, "time_window_hours": 24}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2", // Now in London
			StartedAt: now,
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Equal(t, models.SeverityCritical, result.Violation.Severity)
	assert.Contains(t, result.Violation.Message, "impossible travel")
}

func TestImpossibleTravelEvaluator_PossibleTravel(t *testing.T) {
	now := time.Now().UTC()

	// NYC 8 hours ago - enough time to fly to London
	mock := &mockHistoryQuerierWithLastStream{
		lastStream: &models.WatchHistoryEntry{
			UserName:  "alice",
			IPAddress: "1.1.1.1",
			StartedAt: now.Add(-8 * time.Hour),
		},
	}

	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"},
		},
	}

	evaluator := NewImpossibleTravelEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Impossible Travel",
		Type:   models.RuleTypeImpossibleTravel,
		Config: json.RawMessage(`{"max_speed_km_h": 800, "min_distance_km": 100, "time_window_hours": 24}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2",
			StartedAt: now,
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - travel was possible
}

func TestImpossibleTravelEvaluator_NoPreviousStream(t *testing.T) {
	mock := &mockHistoryQuerierWithLastStream{
		lastStream: nil, // No previous stream
	}

	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"2.2.2.2": {IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"},
		},
	}

	evaluator := NewImpossibleTravelEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "Impossible Travel",
		Type:   models.RuleTypeImpossibleTravel,
		Config: json.RawMessage(`{}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 51.5074, Lng: -0.1278, City: "London", Country: "UK"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - no previous stream to compare
}
