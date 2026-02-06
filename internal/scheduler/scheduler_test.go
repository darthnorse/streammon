package scheduler

import (
	"testing"
	"time"
)

func TestDurationUntil3AM(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "before 3 AM today",
			now:  time.Date(2024, 1, 15, 2, 0, 0, 0, time.Local),
			want: 1 * time.Hour,
		},
		{
			name: "at 3 AM exactly",
			now:  time.Date(2024, 1, 15, 3, 0, 0, 0, time.Local),
			want: 24 * time.Hour,
		},
		{
			name: "after 3 AM",
			now:  time.Date(2024, 1, 15, 15, 30, 0, 0, time.Local),
			want: 11*time.Hour + 30*time.Minute,
		},
		{
			name: "just before midnight",
			now:  time.Date(2024, 1, 15, 23, 59, 0, 0, time.Local),
			want: 3*time.Hour + 1*time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationUntil3AM(tt.now)
			if got != tt.want {
				t.Errorf("durationUntil3AM(%v) = %v, want %v",
					tt.now.Format("15:04"), got, tt.want)
			}
		})
	}
}
