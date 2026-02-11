package store

import (
	"testing"
	"time"
)

func TestParseSQLiteTime(t *testing.T) {
	utc := time.UTC
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		// Empty string returns zero time, no error
		{name: "empty", input: "", want: time.Time{}},

		// RFC3339Nano (modernc.org/sqlite _time_format=sqlite with nanos)
		{name: "rfc3339nano", input: "2024-06-15T10:30:45.123456789Z",
			want: time.Date(2024, 6, 15, 10, 30, 45, 123456789, utc)},

		// RFC3339
		{name: "rfc3339", input: "2024-06-15T10:30:45Z",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// SQLite driver format (space separator, offset timezone)
		{name: "sqlite driver format with nanos", input: "2024-06-15 10:30:45.123456789+00:00",
			want: time.Date(2024, 6, 15, 10, 30, 45, 123456789, utc)},

		// SQLite driver format without fractional seconds
		{name: "sqlite driver format no frac", input: "2024-06-15 10:30:45+00:00",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// Non-UTC offset gets converted to UTC
		{name: "positive offset", input: "2024-06-15 12:30:45+02:00",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},
		{name: "negative offset", input: "2024-06-15 05:30:45-05:00",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// Z suffix (SQLite CURRENT_TIMESTAMP-like)
		{name: "z suffix", input: "2024-06-15 10:30:45Z",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// No timezone — assumed UTC
		{name: "no tz with nanos", input: "2024-06-15 10:30:45.123456789",
			want: time.Date(2024, 6, 15, 10, 30, 45, 123456789, utc)},
		{name: "no tz no frac", input: "2024-06-15 10:30:45",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// T separator without timezone
		{name: "T separator no tz", input: "2024-06-15T10:30:45",
			want: time.Date(2024, 6, 15, 10, 30, 45, 0, utc)},

		// Minute-only precision (SQLite strftime %Y-%m-%d %H:%M)
		{name: "minute precision", input: "2024-06-15 10:30",
			want: time.Date(2024, 6, 15, 10, 30, 0, 0, utc)},

		// Error cases
		{name: "invalid format", input: "not-a-date", wantErr: true},
		{name: "date only", input: "2024-06-15", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSQLiteTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseSQLiteTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if got.Location() != utc && !got.IsZero() {
				t.Errorf("parseSQLiteTime(%q) location = %v, want UTC", tt.input, got.Location())
			}
		})
	}
}
