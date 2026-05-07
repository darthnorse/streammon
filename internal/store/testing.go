package store

import "context"

// ExecForTest executes an arbitrary SQL statement against the store's database.
// It is intended only for seeding test data from external test packages (e.g.
// internal/server tests) that cannot access the unexported db field directly.
func (s *Store) ExecForTest(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}
