package maintenance

import (
	"testing"
)

func TestParseResolutionHeight(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		// Standard resolutions
		{"4K", 2160},
		{"4k", 2160},
		{"2160p", 2160},
		{"2160", 2160},
		{"1080p", 1080},
		{"1080", 1080},
		{"720p", 720},
		{"720", 720},
		{"480p", 480},
		{"480", 480},
		{"360p", 360},
		{"360", 360},
		{"240p", 240},
		{"240", 240},

		// Named resolutions
		{"FHD", 1080},
		{"fhd", 1080},
		{"HD", 720},
		{"hd", 720},
		{"SD", 480},
		{"sd", 480},
		{"UHD", 2160},
		{"uhd", 2160},
		{"8K", 4320},
		{"8k", 4320},

		// Non-standard resolutions (issue #5 fix)
		{"576p", 576},
		{"540p", 540},
		{"544p", 544},
		{"1440p", 1440},

		// Unknown/empty
		{"", 0},
		{"unknown", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseResolutionHeight(tt.input)
			if result != tt.expected {
				t.Errorf("parseResolutionHeight(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify defaults are sensible
	if DefaultDays != 365 {
		t.Errorf("DefaultDays = %d, want 365", DefaultDays)
	}
	if DefaultMaxPercent != 10 {
		t.Errorf("DefaultMaxPercent = %d, want 10", DefaultMaxPercent)
	}
	if DefaultMaxHeight != 720 {
		t.Errorf("DefaultMaxHeight = %d, want 720", DefaultMaxHeight)
	}
	if DefaultMinSizeGB != 10.0 {
		t.Errorf("DefaultMinSizeGB = %f, want 10.0", DefaultMinSizeGB)
	}
}
