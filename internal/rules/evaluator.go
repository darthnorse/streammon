package rules

import (
	"context"
	"math"
	"time"

	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/units"
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
	UnitSystem units.System
}

type EvaluationResult struct {
	Violation *models.RuleViolation
	Signals   []models.ViolationSignal

	// TerminateTarget optionally overrides the auto-terminate target the
	// engine would otherwise derive from the evaluated stream (input.Stream).
	// Only needed by evaluators — e.g. concurrent streams — whose violation
	// isn't about the stream being evaluated but about a different stream
	// (the newest one) that should be terminated instead. Never persisted:
	// it does not live on models.RuleViolation / Violation.Details, so it
	// can't leak through GET /api/violations.
	TerminateTarget *TerminateTarget
}

// TerminateTarget identifies the session an evaluator wants auto-terminated,
// when that target differs from the stream under evaluation.
type TerminateTarget struct {
	ServerID        int64
	SessionID       string
	PlexSessionUUID string
}

type GeoResolver interface {
	Lookup(ctx context.Context, ip string) (*models.GeoResult, error)
}

type ServerResolver interface {
	GetServer(id int64) (media.MediaServer, bool)
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

// formatTimeWindow converts hours into a human-readable time value and unit.
func formatTimeWindow(hours int) (int, string) {
	if hours == 1 {
		return 1, "hour"
	}
	if hours%24 != 0 {
		return hours, "hours"
	}
	days := hours / 24
	if days%7 == 0 {
		weeks := days / 7
		if weeks == 1 {
			return 1, "week"
		}
		return weeks, "weeks"
	}
	if days == 1 {
		return 1, "day"
	}
	return days, "days"
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
