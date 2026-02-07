package units

import "testing"

func TestFormatDistance(t *testing.T) {
	tests := []struct {
		name string
		km   float64
		sys  System
		want string
	}{
		{"metric", 100, Metric, "100 km"},
		{"imperial", 100, Imperial, "62 mi"},
		{"metric zero", 0, Metric, "0 km"},
		{"imperial zero", 0, Imperial, "0 mi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDistance(tt.km, tt.sys)
			if got != tt.want {
				t.Errorf("FormatDistance(%v, %v) = %q, want %q", tt.km, tt.sys, got, tt.want)
			}
		})
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name string
		kmh  float64
		sys  System
		want string
	}{
		{"metric", 100, Metric, "100 km/h"},
		{"imperial", 100, Imperial, "62 mph"},
		{"metric zero", 0, Metric, "0 km/h"},
		{"imperial zero", 0, Imperial, "0 mph"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpeed(tt.kmh, tt.sys)
			if got != tt.want {
				t.Errorf("FormatSpeed(%v, %v) = %q, want %q", tt.kmh, tt.sys, got, tt.want)
			}
		})
	}
}

func TestFormatDistanceValue(t *testing.T) {
	tests := []struct {
		name string
		km   float64
		sys  System
		want float64
	}{
		{"metric", 100, Metric, 100},
		{"imperial", 100, Imperial, 62.1371},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDistanceValue(tt.km, tt.sys)
			if got < tt.want-0.001 || got > tt.want+0.001 {
				t.Errorf("FormatDistanceValue(%v, %v) = %v, want %v", tt.km, tt.sys, got, tt.want)
			}
		})
	}
}

func TestFormatSpeedValue(t *testing.T) {
	tests := []struct {
		name string
		kmh  float64
		sys  System
		want float64
	}{
		{"metric", 100, Metric, 100},
		{"imperial", 100, Imperial, 62.1371},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpeedValue(tt.kmh, tt.sys)
			if got < tt.want-0.001 || got > tt.want+0.001 {
				t.Errorf("FormatSpeedValue(%v, %v) = %v, want %v", tt.kmh, tt.sys, got, tt.want)
			}
		})
	}
}

func TestDistanceUnit(t *testing.T) {
	tests := []struct {
		sys  System
		want string
	}{
		{Metric, "km"},
		{Imperial, "mi"},
	}
	for _, tt := range tests {
		t.Run(string(tt.sys), func(t *testing.T) {
			got := DistanceUnit(tt.sys)
			if got != tt.want {
				t.Errorf("DistanceUnit(%v) = %q, want %q", tt.sys, got, tt.want)
			}
		})
	}
}

func TestSpeedUnit(t *testing.T) {
	tests := []struct {
		sys  System
		want string
	}{
		{Metric, "km/h"},
		{Imperial, "mph"},
	}
	for _, tt := range tests {
		t.Run(string(tt.sys), func(t *testing.T) {
			got := SpeedUnit(tt.sys)
			if got != tt.want {
				t.Errorf("SpeedUnit(%v) = %q, want %q", tt.sys, got, tt.want)
			}
		})
	}
}

func TestParseSystem(t *testing.T) {
	tests := []struct {
		input string
		want  System
	}{
		{"metric", Metric},
		{"imperial", Imperial},
		{"unknown", Metric},
		{"", Metric},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseSystem(tt.input)
			if got != tt.want {
				t.Errorf("ParseSystem(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"metric", true},
		{"imperial", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValid(tt.input)
			if got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
