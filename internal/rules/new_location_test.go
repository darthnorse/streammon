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

type mockHistoryQuerierWithIPs struct {
	mockHistoryQuerier
	distinctIPs []string
}

func (m *mockHistoryQuerierWithIPs) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	return m.distinctIPs, nil
}

func TestNewLocationEvaluator_NewLocation(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{"1.1.1.1"}, // User only has history from NYC
	}
	evaluator := NewNewLocationEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Location Alert",
		Type:   models.RuleTypeNewLocation,
		Config: json.RawMessage(`{"notify_on_new": true, "min_distance_km": 50}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2", // Streaming from LA (far from NYC)
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Violation)
	assert.Contains(t, result.Violation.Message, "new location")
}

func TestNewLocationEvaluator_NearbyLocation(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"1.1.1.2": {IP: "1.1.1.2", Lat: 40.7580, Lng: -73.9855, City: "Manhattan", Country: "US"}, // ~5km away
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{"1.1.1.1"},
	}
	evaluator := NewNewLocationEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Location Alert",
		Type:   models.RuleTypeNewLocation,
		Config: json.RawMessage(`{"notify_on_new": true, "min_distance_km": 50}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "1.1.1.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "1.1.1.2", Lat: 40.7580, Lng: -73.9855, City: "Manhattan", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - location is nearby
}

func TestNewLocationEvaluator_HouseholdExempt(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{"1.1.1.1"},
	}
	evaluator := NewNewLocationEvaluator(geoResolver, mock)

	rule := &models.Rule{
		ID:     1,
		Name:   "New Location Alert",
		Type:   models.RuleTypeNewLocation,
		Config: json.RawMessage(`{"notify_on_new": true, "min_distance_km": 50, "exempt_household": true}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		Households: []models.HouseholdLocation{
			{IPAddress: "2.2.2.2", Trusted: true},
		},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - household exempt
}
