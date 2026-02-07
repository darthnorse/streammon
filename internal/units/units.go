package units

import "fmt"

type System string

const (
	Metric   System = "metric"
	Imperial System = "imperial"
)

const (
	kmToMiles = 0.621371
	kmhToMph  = 0.621371
)

func FormatDistance(km float64, sys System) string {
	if sys == Imperial {
		miles := km * kmToMiles
		return fmt.Sprintf("%.0f mi", miles)
	}
	return fmt.Sprintf("%.0f km", km)
}

func FormatSpeed(kmh float64, sys System) string {
	if sys == Imperial {
		mph := kmh * kmhToMph
		return fmt.Sprintf("%.0f mph", mph)
	}
	return fmt.Sprintf("%.0f km/h", kmh)
}

func FormatDistanceValue(km float64, sys System) float64 {
	if sys == Imperial {
		return km * kmToMiles
	}
	return km
}

func FormatSpeedValue(kmh float64, sys System) float64 {
	if sys == Imperial {
		return kmh * kmhToMph
	}
	return kmh
}

func DistanceUnit(sys System) string {
	if sys == Imperial {
		return "mi"
	}
	return "km"
}

func SpeedUnit(sys System) string {
	if sys == Imperial {
		return "mph"
	}
	return "km/h"
}

func ParseSystem(s string) System {
	if s == "imperial" {
		return Imperial
	}
	return Metric
}

func IsValid(s string) bool {
	return s == "metric" || s == "imperial"
}
