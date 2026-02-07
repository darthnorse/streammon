package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
	"streammon/internal/units"
)

type ImpossibleTravelEvaluator struct {
	geoResolver GeoResolver
	store       HistoryQuerier
}

func NewImpossibleTravelEvaluator(resolver GeoResolver, store HistoryQuerier) *ImpossibleTravelEvaluator {
	return &ImpossibleTravelEvaluator{geoResolver: resolver, store: store}
}

func (e *ImpossibleTravelEvaluator) Type() models.RuleType {
	return models.RuleTypeImpossibleTravel
}

func (e *ImpossibleTravelEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil || input.Stream.IPAddress == "" {
		return nil, nil
	}

	var config models.ImpossibleTravelConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	stream := input.Stream

	// Get previous stream
	prevStream, err := e.store.GetLastStreamBeforeTime(stream.UserName, stream.StartedAt, config.TimeWindowHours)
	if err != nil {
		return nil, fmt.Errorf("getting previous stream: %w", err)
	}
	if prevStream == nil {
		return nil, nil // No previous stream to compare
	}

	// Same IP - no travel
	if prevStream.IPAddress == stream.IPAddress {
		return nil, nil
	}

	// Get current geo
	currentGeo := input.GeoData
	if currentGeo == nil {
		currentGeo, err = e.geoResolver.Lookup(ctx, stream.IPAddress)
		if err != nil || currentGeo == nil {
			return nil, nil
		}
	}

	// Get previous geo
	prevGeo, err := e.geoResolver.Lookup(ctx, prevStream.IPAddress)
	if err != nil || prevGeo == nil {
		return nil, nil
	}

	// Calculate distance and time
	distance := HaversineDistance(currentGeo.Lat, currentGeo.Lng, prevGeo.Lat, prevGeo.Lng)
	if distance < config.MinDistanceKm {
		return nil, nil // Not far enough to care
	}

	timeDelta := stream.StartedAt.Sub(prevStream.StartedAt)
	if timeDelta <= 0 {
		return nil, nil
	}

	velocity := distance / timeDelta.Hours()
	if velocity <= config.MaxSpeedKmH {
		return nil, nil // Travel was possible
	}

	severity := determineSeverityByVelocity(velocity, config.MaxSpeedKmH)

	distStr := units.FormatDistance(distance, input.UnitSystem)
	velStr := units.FormatSpeed(velocity, input.UnitSystem)

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: stream.UserName,
		Severity: severity,
		Message: fmt.Sprintf("impossible travel detected: %s to %s (%s in %.1f hours = %s)",
			prevGeo.City, currentGeo.City, distStr, timeDelta.Hours(), velStr),
		Details: map[string]interface{}{
			"from_city":     prevGeo.City,
			"from_country":  prevGeo.Country,
			"to_city":       currentGeo.City,
			"to_country":    currentGeo.Country,
			"distance_km":   distance,
			"time_hours":    timeDelta.Hours(),
			"velocity_kmh":  velocity,
			"max_speed_kmh": config.MaxSpeedKmH,
		},
		ConfidenceScore: calculateTravelConfidence(velocity, config.MaxSpeedKmH),
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: violation,
		Signals: []models.ViolationSignal{
			{Name: "velocity_kmh", Weight: 0.6, Value: velocity},
			{Name: "distance_km", Weight: 0.4, Value: distance},
		},
	}, nil
}

func determineSeverityByVelocity(velocity, maxSpeed float64) models.Severity {
	ratio := velocity / maxSpeed
	if ratio >= 3 {
		return models.SeverityCritical
	}
	if ratio >= 2 {
		return models.SeverityWarning
	}
	return models.SeverityInfo
}

func calculateTravelConfidence(velocity, maxSpeed float64) float64 {
	ratio := velocity / maxSpeed
	confidence := 70 + (ratio-1)*15
	if confidence > 100 {
		return 100
	}
	return confidence
}
