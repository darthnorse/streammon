package mediautil

import "testing"

func TestHeightFromWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{"4K UHD", 3840, 2160},
		{"DCI 4K", 4096, 2160},
		{"1080p", 1920, 1080},
		{"720p", 1280, 720},
		{"SD widescreen 1024", 1024, 480},
		{"480p threshold", 720, 480},
		{"sub-SD", 640, 0},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HeightFromWidth(tt.width); got != tt.want {
				t.Errorf("HeightFromWidth(%d) = %d, want %d", tt.width, got, tt.want)
			}
		})
	}
}
