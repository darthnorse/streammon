package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"streammon/internal/maintenance"
	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
)

const DefaultSyncTimeout = 6 * time.Hour

type Scheduler struct {
	store       *store.Store
	poller      *poller.Poller
	syncTimeout time.Duration

	startOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
}

type Option func(*Scheduler)

func WithSyncTimeout(d time.Duration) Option {
	return func(s *Scheduler) {
		s.syncTimeout = d
	}
}

func New(s *store.Store, p *poller.Poller, opts ...Option) *Scheduler {
	sch := &Scheduler{
		store:       s,
		poller:      p,
		syncTimeout: DefaultSyncTimeout,
		done:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(sch)
	}
	return sch
}

// Start runs the scheduler: immediate sync on startup, then daily at 3 AM local time.
func (sch *Scheduler) Start(ctx context.Context) {
	sch.startOnce.Do(func() {
		ctx, sch.cancel = context.WithCancel(ctx)
		go sch.run(ctx)
	})
}

func (sch *Scheduler) Stop() {
	if sch.cancel != nil {
		sch.cancel()
		<-sch.done
	}
}

func (sch *Scheduler) run(ctx context.Context) {
	defer close(sch.done)

	if err := sch.SyncAll(ctx); err != nil {
		log.Printf("scheduler: initial sync failed: %v", err)
	}

	// Clean expired sessions on startup
	sch.cleanupSessions()

	syncTicker := time.NewTicker(durationUntil3AM(time.Now()))
	defer syncTicker.Stop()

	// Session cleanup every hour
	sessionTicker := time.NewTicker(1 * time.Hour)
	defer sessionTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-syncTicker.C:
			if err := sch.SyncAll(ctx); err != nil {
				log.Printf("scheduler: daily sync failed: %v", err)
			}
			// Recalculate to handle DST transitions
			syncTicker.Reset(durationUntil3AM(time.Now()))
		case <-sessionTicker.C:
			sch.cleanupSessions()
		}
	}
}

// cleanupSessions removes expired sessions from the database
func (sch *Scheduler) cleanupSessions() {
	deleted, err := sch.store.DeleteExpiredSessions()
	if err != nil {
		log.Printf("scheduler: session cleanup failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("scheduler: cleaned up %d expired sessions", deleted)
	}
}

func (sch *Scheduler) SyncAll(ctx context.Context) error {
	log.Println("scheduler: starting library sync")
	startTime := time.Now().UTC()

	// Phase 1: Sync all libraries
	totalLibs, totalItems, syncErrors := sch.syncAllLibraries(ctx)

	// Phase 2: Evaluate all rules now that all libraries are synced
	totalCandidates, evalErrors := sch.evaluateAllRules(ctx)
	totalErrors := syncErrors + evalErrors

	elapsed := time.Since(startTime)
	if totalErrors > 0 {
		log.Printf("scheduler: completed with %d errors - %d libraries, %d items synced, %d candidates found (took %v)",
			totalErrors, totalLibs, totalItems, totalCandidates, elapsed.Round(time.Second))
	} else {
		log.Printf("scheduler: completed - %d libraries, %d items synced, %d candidates found (took %v)",
			totalLibs, totalItems, totalCandidates, elapsed.Round(time.Second))
	}

	return nil
}

func (sch *Scheduler) syncAllLibraries(ctx context.Context) (totalLibs, totalItems, totalErrors int) {
	servers, err := sch.store.ListServers()
	if err != nil {
		log.Printf("scheduler: list servers: %v", err)
		return 0, 0, 1
	}

	for _, srv := range servers {
		if ctx.Err() != nil {
			return totalLibs, totalItems, totalErrors
		}
		if !srv.Enabled {
			continue
		}

		ms, ok := sch.poller.GetServer(srv.ID)
		if !ok {
			log.Printf("scheduler: server %s (ID:%d) not in poller, skipping", srv.Name, srv.ID)
			continue
		}

		identity := serverIdentity{URL: srv.URL, Type: srv.Type, MachineID: srv.MachineID}

		libs, err := ms.GetLibraries(ctx)
		if err != nil {
			log.Printf("scheduler: get libraries for %s: %v", srv.Name, err)
			totalErrors++
			continue
		}

		for _, lib := range libs {
			if ctx.Err() != nil {
				return totalLibs, totalItems, totalErrors
			}
			if lib.Type != models.LibraryTypeMovie && lib.Type != models.LibraryTypeShow {
				continue
			}

			itemCount, syncErr := sch.syncLibrary(ctx, srv.ID, srv.Name, lib.ID, lib.Name, ms, identity)
			if syncErr != nil {
				log.Printf("scheduler: sync %s/%s: %v", srv.Name, lib.Name, syncErr)
				totalErrors++
				continue
			}

			totalLibs++
			totalItems += itemCount
		}
	}

	log.Printf("scheduler: phase 1 complete - synced %d libraries, %d items", totalLibs, totalItems)
	return totalLibs, totalItems, totalErrors
}

func (sch *Scheduler) evaluateAllRules(ctx context.Context) (totalCandidates, totalErrors int) {
	rules, err := sch.store.ListAllMaintenanceRules(ctx)
	if err != nil {
		log.Printf("scheduler: list all maintenance rules: %v", err)
		return 0, 1
	}

	if len(rules) == 0 {
		return 0, 0
	}

	evaluator := maintenance.NewEvaluator(sch.store)

	for _, rule := range rules {
		if ctx.Err() != nil {
			return totalCandidates, totalErrors
		}

		candidates, evalErr := evaluator.EvaluateRule(ctx, &rule)
		if evalErr != nil {
			log.Printf("scheduler: evaluate rule %d (%s): %v", rule.ID, rule.Name, evalErr)
			totalErrors++
			continue
		}

		if err := sch.store.BatchUpsertCandidates(ctx, rule.ID, candidates); err != nil {
			log.Printf("scheduler: upsert candidates for rule %d: %v", rule.ID, err)
			totalErrors++
			continue
		}

		if len(candidates) > 0 {
			log.Printf("scheduler: rule %d (%s): found %d candidates", rule.ID, rule.Name, len(candidates))
		}
		totalCandidates += len(candidates)
	}

	log.Printf("scheduler: phase 2 complete - evaluated %d rules, %d candidates found", len(rules), totalCandidates)
	return totalCandidates, totalErrors
}

// serverIdentity captures the fields that define a server's "identity" for sync purposes.
// If any of these change during a sync, the sync should abort to avoid writing stale data.
type serverIdentity struct {
	URL       string
	Type      models.ServerType
	MachineID string
}

func (sch *Scheduler) syncLibrary(ctx context.Context, serverID int64, serverName, libraryID, libraryName string, ms media.MediaServer, originalIdentity serverIdentity) (int, error) {
	syncCtx, cancel := context.WithTimeout(ctx, sch.syncTimeout)
	defer cancel()

	syncStart := time.Now().UTC()

	items, err := ms.GetLibraryItems(syncCtx, libraryID)
	if err != nil {
		return 0, err
	}

	// Check if server identity changed during fetch - abort if so to avoid writing stale data
	currentServer, err := sch.store.GetServer(serverID)
	if err != nil {
		return 0, err
	}
	if currentServer.URL != originalIdentity.URL ||
		currentServer.Type != originalIdentity.Type ||
		currentServer.MachineID != originalIdentity.MachineID {
		log.Printf("scheduler: server %s identity changed during sync, aborting to prevent stale data", serverName)
		return 0, nil
	}

	count, err := sch.store.UpsertLibraryItems(syncCtx, items)
	if err != nil {
		return 0, err
	}

	deleted, err := sch.store.DeleteStaleLibraryItems(syncCtx, serverID, libraryID, syncStart)
	if err != nil {
		log.Printf("scheduler: warning: delete stale items for %s/%s failed: %v", serverName, libraryName, err)
	} else if deleted > 0 {
		log.Printf("scheduler: removed %d stale items from %s/%s", deleted, serverName, libraryName)
	}

	return count, nil
}

// durationUntil3AM uses local time so the job runs at 3 AM in the server's timezone.
func durationUntil3AM(now time.Time) time.Duration {
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())

	if !now.Before(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}

	return next3AM.Sub(now)
}
