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
	err         error
}

func (m *mockHistoryQuerierWithIPs) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
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

func TestNewLocationEvaluator_NoHistory(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"2.2.2.2": {IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{}, // First-time user, no history
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
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - first stream, let them establish history
}

func TestNewLocationEvaluator_SameIP(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{"1.1.1.1"}, // Same IP in history
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
			IPAddress: "1.1.1.1", // Same IP as in history
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "1.1.1.1", Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - same IP, not new
}

func TestNewLocationEvaluator_NotifyDisabled(t *testing.T) {
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
		Config: json.RawMessage(`{"notify_on_new": false, "min_distance_km": 50}`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "alice",
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - notifications disabled
}

func TestNewLocationEvaluator_AllGeoLookupsFail(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"2.2.2.2": {IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
			// 1.1.1.1 is NOT in the results, so lookup will return nil
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		distinctIPs: []string{"1.1.1.1"}, // Historical IP with no geo data
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
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.NoError(t, err)
	assert.Nil(t, result) // No violation - can't determine distance when all geo lookups fail
}

func TestNewLocationEvaluator_StoreError(t *testing.T) {
	geoResolver := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"2.2.2.2": {IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	mock := &mockHistoryQuerierWithIPs{
		err: assert.AnError,
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
			IPAddress: "2.2.2.2",
			StartedAt: time.Now().UTC(),
		},
		GeoData: &models.GeoResult{IP: "2.2.2.2", Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
	}

	result, err := evaluator.Evaluate(context.Background(), rule, input)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "getting historical IPs")
}
