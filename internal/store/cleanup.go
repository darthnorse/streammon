package store

import (
	"fmt"
	"log"
)

const zombieCleanupKey = "migration.zombie_cleanup_done"

// CleanupZombieSessions caps stopped_at for zombie sessions where wall time
// vastly exceeds actual watched time. Runs once, guarded by a setting flag.
func (s *Store) CleanupZombieSessions() error {
	done, err := s.GetSetting(zombieCleanupKey)
	if err != nil {
		return fmt.Errorf("checking zombie cleanup flag: %w", err)
	}
	if done == "1" {
		return nil
	}

	result, err := s.db.Exec(`
		UPDATE watch_history
		SET stopped_at = datetime(started_at, '+' || (watched_ms / 1000 + 300) || ' seconds')
		WHERE (julianday(stopped_at) - julianday(started_at)) * 86400 > watched_ms / 1000 * 5 + 3600
		  AND watched_ms > 0
	`)
	if err != nil {
		return fmt.Errorf("cleaning zombie sessions: %w", err)
	}

	// Also fix zero-progress zombies: sessions with no watched time but lingering for > 1 hour
	resultZero, err := s.db.Exec(`
		UPDATE watch_history
		SET stopped_at = started_at
		WHERE watched_ms = 0
		  AND (julianday(stopped_at) - julianday(started_at)) * 86400 > 3600
	`)
	if err != nil {
		return fmt.Errorf("cleaning zero-progress zombie sessions: %w", err)
	}

	fixed, _ := result.RowsAffected()
	fixedZero, _ := resultZero.RowsAffected()
	if total := fixed + fixedZero; total > 0 {
		log.Printf("zombie cleanup: fixed %d sessions with inflated wall times (%d zero-progress)", total, fixedZero)
	}

	return s.SetSetting(zombieCleanupKey, "1")
}
