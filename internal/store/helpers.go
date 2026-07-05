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

// writeChunkSize bounds how many rows a single write transaction touches in
// bulk store operations (BatchUpsertCandidates, InsertHistoryBatch). SQLite
// allows only one writer transaction at a time; a transaction covering
// thousands of rows holds that lock for its whole duration, delaying any
// other writer (notably the poller's session/history inserts) until commit
// or busy_timeout. Chunking commits periodically so the lock is released
// between chunks. A var (not const) so tests can shrink it to exercise
// multi-chunk behavior without seeding hundreds of rows.
var writeChunkSize = 500

// chunkSlice splits items into consecutive slices of at most size elements,
// preserving order. Returns nil for an empty input.
func chunkSlice[T any](items []T, size int) [][]T {
	if len(items) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(items)
	}
	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}
