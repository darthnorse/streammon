package server

import (
	"context"
	"log"
	"sync"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tautulli"
)

// enrichRateInterval is the minimum delay between successive Tautulli API calls
// during background enrichment (~10 requests/second).
const enrichRateInterval = 100 * time.Millisecond

type enrichmentState struct {
	mu        sync.RWMutex
	wg        sync.WaitGroup
	running   bool
	processed int
	total     int
	serverID  int64
	cancel    context.CancelFunc
}

type enrichmentStatusResponse struct {
	Running   bool  `json:"running"`
	Processed int   `json:"processed"`
	Total     int   `json:"total"`
	ServerID  int64 `json:"server_id"`
}

func (e *enrichmentState) status() enrichmentStatusResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return enrichmentStatusResponse{
		Running:   e.running,
		Processed: e.processed,
		Total:     e.total,
		ServerID:  e.serverID,
	}
}

// start atomically checks if enrichment is already running and, if not, starts
// a background goroutine. Returns false if enrichment was already running.
func (e *enrichmentState) start(ctx context.Context, st *store.Store, client *tautulli.Client, serverID int64, total int) bool {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return false
	}
	enrichCtx, cancel := context.WithCancel(ctx)
	e.running = true
	e.processed = 0
	e.total = total
	e.serverID = serverID
	e.cancel = cancel
	e.mu.Unlock()

	e.wg.Add(1)
	go e.run(enrichCtx, st, client, serverID)
	return true
}

// stop cancels a running enrichment. No-op if not running.
func (e *enrichmentState) stop() {
	e.mu.RLock()
	cancel := e.cancel
	e.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

// Wait blocks until any running enrichment goroutine finishes.
func (e *enrichmentState) Wait() {
	e.wg.Wait()
}

func (e *enrichmentState) run(ctx context.Context, st *store.Store, client *tautulli.Client, serverID int64) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("enrichment: panic recovered: %v", r)
		}
		e.mu.Lock()
		if e.cancel != nil {
			e.cancel()
			e.cancel = nil
		}
		e.running = false
		e.mu.Unlock()
	}()

	limiter := time.NewTicker(enrichRateInterval)
	defer limiter.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		refs, err := st.ListUnenrichedHistory(ctx, serverID, 100)
		if err != nil {
			log.Printf("enrichment: list error: %v", err)
			return
		}
		if len(refs) == 0 {
			return
		}

		for _, ref := range refs {
			select {
			case <-ctx.Done():
				return
			case <-limiter.C:
			}

			streamData, err := client.GetStreamData(ctx, int(ref.RefID))
			if err != nil {
				log.Printf("enrichment: GetStreamData ref_id=%d: %v", ref.RefID, err)
				if markErr := st.MarkHistoryEnriched(ctx, ref.ID); markErr != nil {
					log.Printf("enrichment: MarkHistoryEnriched id=%d: %v", ref.ID, markErr)
					return
				}
			} else if streamData != nil {
				entry := &models.WatchHistoryEntry{}
				enrichEntryFromStreamData(entry, streamData)
				if err := st.UpdateHistoryEnrichment(ctx, ref.ID, entry); err != nil {
					log.Printf("enrichment: update id=%d: %v", ref.ID, err)
					return
				}
			} else {
				if markErr := st.MarkHistoryEnriched(ctx, ref.ID); markErr != nil {
					log.Printf("enrichment: MarkHistoryEnriched id=%d: %v", ref.ID, markErr)
					return
				}
			}

			e.mu.Lock()
			e.processed++
			e.mu.Unlock()
		}
	}
}
