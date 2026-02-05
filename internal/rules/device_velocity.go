package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
)

type DeviceVelocityEvaluator struct {
	store HistoryQuerier
}

func NewDeviceVelocityEvaluator(store HistoryQuerier) *DeviceVelocityEvaluator {
	return &DeviceVelocityEvaluator{store: store}
}

func (e *DeviceVelocityEvaluator) Type() models.RuleType {
	return models.RuleTypeDeviceVelocity
}

func (e *DeviceVelocityEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.DeviceVelocityConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	stream := input.Stream

	recentDevices, err := e.store.GetRecentDevices(stream.UserName, stream.StartedAt, config.TimeWindowHours)
	if err != nil {
		return nil, fmt.Errorf("getting recent devices: %w", err)
	}

	currentKey := stream.Player + "|" + stream.Platform
	found := false
	for _, d := range recentDevices {
		if d.Player+"|"+d.Platform == currentKey {
			found = true
			break
		}
	}

	deviceCount := len(recentDevices)
	if !found {
		deviceCount++
	}

	if deviceCount <= config.MaxDevicesPerHour {
		return nil, nil
	}

	severity := models.SeverityInfo
	if deviceCount >= config.MaxDevicesPerHour*2 {
		severity = models.SeverityCritical
	} else if deviceCount >= config.MaxDevicesPerHour+2 {
		severity = models.SeverityWarning
	}

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: stream.UserName,
		Severity: severity,
		Message:  fmt.Sprintf("user has used %d devices in the past %d hour(s) (max: %d)", deviceCount, config.TimeWindowHours, config.MaxDevicesPerHour),
		Details: map[string]interface{}{
			"device_count": deviceCount,
			"max_allowed":  config.MaxDevicesPerHour,
			"time_window":  config.TimeWindowHours,
		},
		ConfidenceScore: float64(50 + (deviceCount-config.MaxDevicesPerHour)*15),
		OccurredAt:      time.Now().UTC(),
	}
	if violation.ConfidenceScore > 100 {
		violation.ConfidenceScore = 100
	}

	return &EvaluationResult{
		Violation: violation,
		Signals: []models.ViolationSignal{
			{Name: "device_count", Weight: 0.7, Value: float64(deviceCount)},
			{Name: "max_allowed", Weight: 0.0, Value: float64(config.MaxDevicesPerHour)},
			{Name: "excess", Weight: 0.3, Value: float64(deviceCount - config.MaxDevicesPerHour)},
		},
	}, nil
}
