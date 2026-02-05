package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
)

type ConcurrentStreamsEvaluator struct{}

func NewConcurrentStreamsEvaluator() *ConcurrentStreamsEvaluator {
	return &ConcurrentStreamsEvaluator{}
}

func (e *ConcurrentStreamsEvaluator) Type() models.RuleType {
	return models.RuleTypeConcurrentStreams
}

func (e *ConcurrentStreamsEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.ConcurrentStreamsConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	userName := input.Stream.UserName
	userStreams := filterStreamsByUser(input.AllStreams, userName)
	streamCount := len(userStreams)

	if streamCount <= config.MaxStreams {
		return nil, nil
	}

	if config.ExemptHousehold && allFromHousehold(userStreams, input.Households) {
		return nil, nil
	}

	signals := []models.ViolationSignal{
		{Name: "stream_count", Weight: 0.6, Value: float64(streamCount)},
		{Name: "max_allowed", Weight: 0.0, Value: float64(config.MaxStreams)},
		{Name: "excess", Weight: 0.4, Value: float64(streamCount - config.MaxStreams) * 25},
	}

	confidence := models.CalculateConfidence(signals)
	if confidence < 50 {
		confidence = 50 + float64(streamCount-config.MaxStreams)*10
	}
	if confidence > 100 {
		confidence = 100
	}

	locations := extractLocations(userStreams)
	devices := extractDevices(userStreams)

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: userName,
		Severity: determineSeverity(streamCount, config.MaxStreams),
		Message:  fmt.Sprintf("%d concurrent streams detected (max: %d)", streamCount, config.MaxStreams),
		Details: map[string]interface{}{
			"stream_count": streamCount,
			"max_allowed":  config.MaxStreams,
			"locations":    locations,
			"devices":      devices,
		},
		ConfidenceScore: confidence,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: violation,
		Signals:   signals,
	}, nil
}

func allFromHousehold(streams []models.ActiveStream, households []models.HouseholdLocation) bool {
	if len(households) == 0 {
		return false
	}

	householdIPs := trustedHouseholdIPs(households)
	for _, s := range streams {
		if s.IPAddress == "" || !householdIPs[s.IPAddress] {
			return false
		}
	}
	return true
}

func determineSeverity(actual, max int) models.Severity {
	excess := actual - max
	if excess >= 3 {
		return models.SeverityCritical
	}
	if excess >= 2 {
		return models.SeverityWarning
	}
	return models.SeverityInfo
}

func extractLocations(streams []models.ActiveStream) []string {
	seen := make(map[string]bool)
	var locations []string
	for _, s := range streams {
		if s.IPAddress != "" && !seen[s.IPAddress] {
			seen[s.IPAddress] = true
			locations = append(locations, s.IPAddress)
		}
	}
	return locations
}

func extractDevices(streams []models.ActiveStream) []string {
	seen := make(map[string]bool)
	var devices []string
	for _, s := range streams {
		key := fmt.Sprintf("%s (%s)", s.Player, s.Platform)
		if !seen[key] {
			seen[key] = true
			devices = append(devices, key)
		}
	}
	return devices
}
