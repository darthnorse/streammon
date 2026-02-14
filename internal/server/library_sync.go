package server

import (
	"errors"
	"sync"
	"time"

	"streammon/internal/mediautil"
)

const completedJobTTL = 30 * time.Second

type librarySyncManager struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	active map[string]*librarySyncJob
}

type librarySyncJob struct {
	progress mediautil.SyncProgress
	doneAt   time.Time
	synced   int
	deleted  int
	err      error
}

// tryStart atomically checks whether a sync is already running for the given
// key and, if not, creates a new job. Returns false if a sync is in progress.
func (m *librarySyncManager) tryStart(key, libraryID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, ok := m.active[key]; ok {
		if job.doneAt.IsZero() {
			return false
		}
	}

	m.active[key] = &librarySyncJob{
		progress: mediautil.SyncProgress{
			Phase:   mediautil.PhaseItems,
			Library: libraryID,
		},
	}
	m.wg.Add(1)
	return true
}

// updateProgress atomically replaces the job's progress snapshot.
func (m *librarySyncManager) updateProgress(key string, p mediautil.SyncProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, ok := m.active[key]; ok {
		job.progress = p
	}
}

// finish marks the job as complete and records results.
func (m *librarySyncManager) finish(key string, synced int, deleted int, err error) {
	m.mu.Lock()
	if job, ok := m.active[key]; ok {
		job.doneAt = time.Now().UTC()
		job.synced = synced
		job.deleted = deleted
		job.err = err
	}
	m.mu.Unlock()
	m.wg.Done()
}

// status returns progress for all active and recently-completed jobs.
// Completed jobs older than completedJobTTL are cleaned up.
func (m *librarySyncManager) status() map[string]mediautil.SyncProgress {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	result := make(map[string]mediautil.SyncProgress, len(m.active))

	for key, job := range m.active {
		if !job.doneAt.IsZero() {
			if now.Sub(job.doneAt) > completedJobTTL {
				delete(m.active, key)
				continue
			}
			if job.err != nil {
				var errMsg string
				var se *syncError
				if errors.As(job.err, &se) {
					errMsg = se.message
				} else {
					errMsg = job.err.Error()
				}
				result[key] = mediautil.SyncProgress{
					Phase:   mediautil.PhaseError,
					Library: job.progress.Library,
					Error:   errMsg,
				}
			} else {
				result[key] = mediautil.SyncProgress{
					Phase:   mediautil.PhaseDone,
					Library: job.progress.Library,
					Synced:  job.synced,
					Deleted: job.deleted,
				}
			}
		} else {
			result[key] = job.progress
		}
	}

	return result
}

// Wait blocks until all running sync jobs complete.
func (m *librarySyncManager) Wait() {
	m.wg.Wait()
}
