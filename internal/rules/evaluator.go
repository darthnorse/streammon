package rules

import (
	"context"
	"math"
	"time"

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

// HistoryQuerier provides methods to query watch history for rule evaluation.
type HistoryQuerier interface {
	GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error)
	GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error)
	HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error)
	GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error)
	GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error)
	GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error)
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

// HaversineDistance calculates the distance in km between two lat/lng points.
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
