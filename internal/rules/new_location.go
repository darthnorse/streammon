package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
)

type NewLocationEvaluator struct {
	geoResolver GeoResolver
	store       HistoryQuerier
}

func NewNewLocationEvaluator(resolver GeoResolver, store HistoryQuerier) *NewLocationEvaluator {
	return &NewLocationEvaluator{geoResolver: resolver, store: store}
}

func (e *NewLocationEvaluator) Type() models.RuleType {
	return models.RuleTypeNewLocation
}

func (e *NewLocationEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil || input.Stream.IPAddress == "" {
		return nil, nil
	}

	var config models.NewLocationConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if !config.NotifyOnNew {
		return nil, nil
	}

	stream := input.Stream

	// Check household exemption
	if config.ExemptHousehold {
		householdIPs := trustedHouseholdIPs(input.Households)
		if householdIPs[stream.IPAddress] {
			return nil, nil
		}
	}

	// Get current location
	currentGeo := input.GeoData
	if currentGeo == nil {
		var err error
		currentGeo, err = e.geoResolver.Lookup(ctx, stream.IPAddress)
		if err != nil || currentGeo == nil {
			return nil, nil // Can't determine location
		}
	}

	// Get historical IPs
	historicalIPs, err := e.store.GetUserDistinctIPs(stream.UserName, stream.StartedAt, 100)
	if err != nil {
		return nil, fmt.Errorf("getting historical IPs: %w", err)
	}

	// First stream ever - don't alert (let them establish history)
	if len(historicalIPs) == 0 {
		return nil, nil
	}

	// Check distance to all historical locations
	minDistance := float64(-1)
	for _, ip := range historicalIPs {
		if ip == stream.IPAddress {
			return nil, nil // Same IP, not new
		}

		histGeo, err := e.geoResolver.Lookup(ctx, ip)
		if err != nil || histGeo == nil {
			continue
		}

		dist := HaversineDistance(currentGeo.Lat, currentGeo.Lng, histGeo.Lat, histGeo.Lng)
		if minDistance < 0 || dist < minDistance {
			minDistance = dist
		}

		// Early exit: if we found a location within threshold, no violation possible
		if minDistance >= 0 && minDistance < config.MinDistanceKm {
			return nil, nil
		}
	}

	// All historical IPs failed geo lookup
	if minDistance < 0 {
		return nil, nil
	}

	severity := models.SeverityInfo
	if minDistance >= 500 {
		severity = models.SeverityWarning
	}

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: stream.UserName,
		Severity: severity,
		Message:  fmt.Sprintf("streaming from new location: %s, %s (%.0f km from nearest known location)", currentGeo.City, currentGeo.Country, minDistance),
		Details: map[string]interface{}{
			"city":         currentGeo.City,
			"country":      currentGeo.Country,
			"ip":           stream.IPAddress,
			"min_distance": minDistance,
		},
		ConfidenceScore: 90,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: violation,
		Signals: []models.ViolationSignal{
			{Name: "distance_km", Weight: 0.7, Value: minDistance},
			{Name: "new_location", Weight: 0.3, Value: true},
		},
	}, nil
}
