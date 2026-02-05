package rules

import (
	"context"

	"streammon/internal/models"
)

type Evaluator interface {
	Type() models.RuleType
	Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error)
}

type EvaluationInput struct {
	Stream     *models.ActiveStream
	AllStreams []models.ActiveStream
	Households []models.HouseholdLocation
	GeoData    *models.GeoResult
}

type EvaluationResult struct {
	Violation *models.RuleViolation
	Signals   []models.ViolationSignal
}

type GeoResolver interface {
	Lookup(ctx context.Context, ip string) (*models.GeoResult, error)
}

// trustedHouseholdIPs returns a set of IP addresses from trusted household locations.
func trustedHouseholdIPs(households []models.HouseholdLocation) map[string]bool {
	ips := make(map[string]bool)
	for _, h := range households {
		if h.Trusted && h.IPAddress != "" {
			ips[h.IPAddress] = true
		}
	}
	return ips
}

// filterStreamsByUser returns streams belonging to the specified user.
func filterStreamsByUser(streams []models.ActiveStream, userName string) []models.ActiveStream {
	var result []models.ActiveStream
	for _, s := range streams {
		if s.UserName == userName {
			result = append(result, s)
		}
	}
	return result
}
