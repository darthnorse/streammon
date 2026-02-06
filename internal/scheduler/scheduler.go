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

const DefaultSyncTimeout = 5 * time.Minute

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

	ticker := time.NewTicker(durationUntil3AM(time.Now()))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sch.SyncAll(ctx); err != nil {
				log.Printf("scheduler: daily sync failed: %v", err)
			}
			// Recalculate to handle DST transitions
			ticker.Reset(durationUntil3AM(time.Now()))
		}
	}
}

func (sch *Scheduler) SyncAll(ctx context.Context) error {
	log.Println("scheduler: starting library sync and rule evaluation")
	startTime := time.Now().UTC()

	servers, err := sch.store.ListServers()
	if err != nil {
		return err
	}

	var totalLibs, totalItems, totalCandidates, totalErrors int

	for _, srv := range servers {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !srv.Enabled {
			continue
		}

		ms, ok := sch.poller.GetServer(srv.ID)
		if !ok {
			log.Printf("scheduler: server %s (ID:%d) not in poller, skipping", srv.Name, srv.ID)
			continue
		}

		libs, err := ms.GetLibraries(ctx)
		if err != nil {
			log.Printf("scheduler: get libraries for %s: %v", srv.Name, err)
			totalErrors++
			continue
		}

		for _, lib := range libs {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if lib.Type != models.LibraryTypeMovie && lib.Type != models.LibraryTypeShow {
				continue
			}

			itemCount, candidateCount, syncErr := sch.syncLibrary(ctx, srv.ID, srv.Name, lib.ID, lib.Name, ms)
			if syncErr != nil {
				log.Printf("scheduler: sync %s/%s: %v", srv.Name, lib.Name, syncErr)
				totalErrors++
				continue
			}

			totalLibs++
			totalItems += itemCount
			totalCandidates += candidateCount
		}
	}

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

func (sch *Scheduler) syncLibrary(ctx context.Context, serverID int64, serverName, libraryID, libraryName string, ms media.MediaServer) (int, int, error) {
	syncCtx, cancel := context.WithTimeout(ctx, sch.syncTimeout)
	defer cancel()

	syncStart := time.Now().UTC()

	items, err := ms.GetLibraryItems(syncCtx, libraryID)
	if err != nil {
		return 0, 0, err
	}

	count, err := sch.store.UpsertLibraryItems(syncCtx, items)
	if err != nil {
		return 0, 0, err
	}

	deleted, err := sch.store.DeleteStaleLibraryItems(syncCtx, serverID, libraryID, syncStart)
	if err != nil {
		log.Printf("scheduler: warning: delete stale items for %s/%s failed: %v", serverName, libraryName, err)
	} else if deleted > 0 {
		log.Printf("scheduler: removed %d stale items from %s/%s", deleted, serverName, libraryName)
	}

	rules, err := sch.store.ListMaintenanceRules(syncCtx, serverID, libraryID)
	if err != nil {
		log.Printf("scheduler: list rules for %s/%s: %v", serverName, libraryName, err)
		return count, 0, nil
	}

	var totalCandidates int
	evaluator := maintenance.NewEvaluator(sch.store)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		candidates, evalErr := evaluator.EvaluateRule(syncCtx, &rule)
		if evalErr != nil {
			log.Printf("scheduler: evaluate rule %d (%s) for %s/%s: %v", rule.ID, rule.Name, serverName, libraryName, evalErr)
			continue
		}

		if err := sch.store.BatchUpsertCandidates(syncCtx, rule.ID, maintenance.ToBatch(candidates)); err != nil {
			log.Printf("scheduler: upsert candidates for rule %d: %v", rule.ID, err)
			continue
		}

		if len(candidates) > 0 {
			log.Printf("scheduler: rule %d (%s): found %d candidates", rule.ID, rule.Name, len(candidates))
		}
		totalCandidates += len(candidates)
	}

	return count, totalCandidates, nil
}

// durationUntil3AM uses local time so the job runs at 3 AM in the server's timezone.
func durationUntil3AM(now time.Time) time.Duration {
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())

	if !now.Before(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}

	return next3AM.Sub(now)
}
