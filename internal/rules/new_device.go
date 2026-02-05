package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
)

type NewDeviceEvaluator struct {
	store HistoryQuerier
}

func NewNewDeviceEvaluator(store HistoryQuerier) *NewDeviceEvaluator {
	return &NewDeviceEvaluator{store: store}
}

func (e *NewDeviceEvaluator) Type() models.RuleType {
	return models.RuleTypeNewDevice
}

func (e *NewDeviceEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.NewDeviceConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if !config.NotifyOnNew {
		return nil, nil
	}

	stream := input.Stream
	used, err := e.store.HasDeviceBeenUsed(stream.UserName, stream.Player, stream.Platform, stream.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("checking device history: %w", err)
	}

	if used {
		return nil, nil
	}

	deviceName := fmt.Sprintf("%s (%s)", stream.Player, stream.Platform)

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: stream.UserName,
		Severity: models.SeverityInfo,
		Message:  fmt.Sprintf("streaming from new device: %s", deviceName),
		Details: map[string]interface{}{
			"player":   stream.Player,
			"platform": stream.Platform,
			"device":   deviceName,
		},
		ConfidenceScore: 100,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: violation,
		Signals: []models.ViolationSignal{
			{Name: "new_device", Weight: 1.0, Value: true},
		},
	}, nil
}
