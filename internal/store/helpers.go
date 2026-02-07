package store

import "strings"

// escapeLikePattern escapes SQL LIKE wildcard characters (%, _, \)
// for use with the ESCAPE '\' clause in SQLite queries.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
