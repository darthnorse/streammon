package store

import (
	"fmt"
	"time"
)

// parseSQLiteTime parses a timestamp string returned by SQLite.
// Handles formats produced by the modernc.org/sqlite driver, SQLite built-in
// functions (datetime, strftime), and RFC3339. Times without an explicit
// timezone are assumed UTC.
func parseSQLiteTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	for _, f := range sqliteTimeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}

	for _, f := range sqliteTimeFormatsNoTZ {
		if t, err := time.ParseInLocation(f, s, time.UTC); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %q", s)
}

var sqliteTimeFormats = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05Z",
}

var sqliteTimeFormatsNoTZ = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04",
}
