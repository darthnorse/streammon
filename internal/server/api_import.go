package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

const maxDurationMs = 24 * 60 * 60 * 1000

type importRequest struct {
	ServerID int64 `json:"server_id"`
}

type importProgressEvent struct {
	Type         string `json:"type"`
	Processed    int    `json:"processed"`
	Total        int    `json:"total"`
	Inserted     int    `json:"inserted"`
	Skipped      int    `json:"skipped"`
	Consolidated int    `json:"consolidated"`
	Error        string `json:"error,omitempty"`
}

func clampMs(v, max int64) int64 {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}

type importTracker struct {
	label        string
	serverID     int64
	send         func(importProgressEvent)
	total        int
	inserted     int
	skipped      int
	consolidated int
	processed    int
}

func newImportTracker(label string, serverID int64, w http.ResponseWriter, flusher http.Flusher) *importTracker {
	return &importTracker{
		label:    label,
		serverID: serverID,
		send: func(event importProgressEvent) {
			data, err := json.Marshal(event)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		},
	}
}

func (t *importTracker) insertBatch(ctx context.Context, st *store.Store, entries []*models.WatchHistoryEntry, total int) error {
	inserted, skipped, consolidated, err := st.InsertHistoryBatch(ctx, entries)
	if err != nil {
		return err
	}
	t.inserted += inserted
	t.skipped += skipped
	t.consolidated += consolidated
	t.total = total
	t.processed += len(entries)
	t.send(importProgressEvent{
		Type:         "progress",
		Processed:    t.processed,
		Total:        t.total,
		Inserted:     t.inserted,
		Skipped:      t.skipped,
		Consolidated: t.consolidated,
	})
	return nil
}

func (t *importTracker) fail(err error) {
	log.Printf("%s import error: %v (imported %d, skipped %d, consolidated %d)",
		t.label, err, t.inserted, t.skipped, t.consolidated)
	t.send(importProgressEvent{
		Type:         "error",
		Processed:    t.processed,
		Total:        t.total,
		Inserted:     t.inserted,
		Skipped:      t.skipped,
		Consolidated: t.consolidated,
		Error:        "import failed, check server logs",
	})
}

func (t *importTracker) complete() {
	log.Printf("%s import completed: %d inserted, %d skipped, %d consolidated, server_id=%d",
		t.label, t.inserted, t.skipped, t.consolidated, t.serverID)
	t.send(importProgressEvent{
		Type:         "complete",
		Processed:    t.processed,
		Total:        t.total,
		Inserted:     t.inserted,
		Skipped:      t.skipped,
		Consolidated: t.consolidated,
	})
}

type importStreamer func(ctx context.Context, serverID int64, pageSize int,
	handler func(entries []*models.WatchHistoryEntry, total int) error) error

func (s *Server) handleHistoryImport(
	getConfig func() (store.IntegrationConfig, error),
	makeStreamer func(cfg store.IntegrationConfig) (importStreamer, error),
	label string,
) http.HandlerFunc {
	var mu sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if !mu.TryLock() {
			writeError(w, http.StatusConflict, label+" import already in progress")
			return
		}
		defer mu.Unlock()

		r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
		var req importRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if req.ServerID == 0 {
			writeError(w, http.StatusBadRequest, "server_id is required")
			return
		}

		srv, err := s.store.GetServer(req.ServerID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if srv.DeletedAt != nil {
			writeError(w, http.StatusBadRequest, "server has been deleted")
			return
		}

		cfg, err := getConfig()
		if err != nil {
			log.Printf("ERROR %s import: getConfig: %v", label, err)
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}

		if !cfg.HasCredentials() {
			writeError(w, http.StatusBadRequest, label+" settings not configured")
			return
		}

		streamer, err := makeStreamer(cfg)
		if err != nil {
			log.Printf("ERROR %s import: makeStreamer: %v", label, err)
			writeError(w, http.StatusBadGateway, "failed to connect to "+label+" server")
			return
		}

		flusher, ok := sseFlusher(w)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()

		tracker := newImportTracker(label, req.ServerID, w, flusher)

		err = streamer(ctx, req.ServerID, 1000, func(entries []*models.WatchHistoryEntry, total int) error {
			return tracker.insertBatch(ctx, s.store, entries, total)
		})

		if err != nil {
			tracker.fail(err)
			return
		}

		tracker.complete()
	}
}
