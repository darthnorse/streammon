package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/media"
	"streammon/internal/media/plex"
	"streammon/internal/models"
)

type autoSyncState struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	active map[int64]bool
}

func (a *autoSyncState) tryStart(serverID int64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active[serverID] {
		return false
	}
	if a.active == nil {
		a.active = make(map[int64]bool)
	}
	a.active[serverID] = true
	a.wg.Add(1)
	return true
}

func (a *autoSyncState) finish(serverID int64) {
	a.mu.Lock()
	delete(a.active, serverID)
	a.mu.Unlock()
	a.wg.Done()
}

func (a *autoSyncState) Wait() {
	a.wg.Wait()
}

func parseServerID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal")
}

func (s *Server) syncServerToPoller(srv *models.Server) {
	if s.poller == nil {
		return
	}
	s.poller.RemoveServer(srv.ID)
	if srv.Enabled {
		ms, err := media.NewMediaServer(*srv)
		if err != nil {
			log.Printf("failed to create media adapter for %s: %v", srv.Name, err)
			return
		}
		s.poller.AddServer(srv.ID, ms)
	}
}

// redactServerForViewer removes sensitive infrastructure details from server for non-admin users
func redactServerForViewer(srv models.Server) models.Server {
	srv.URL = ""
	srv.MachineID = ""
	return srv
}

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	isAdmin := user != nil && user.Role == models.RoleAdmin

	// Admins see all servers (including deleted) for settings management;
	// viewers only see active servers.
	var servers []models.Server
	var err error
	if isAdmin {
		servers, err = s.store.ListAllServers()
	} else {
		servers, err = s.store.ListServers()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if !isAdmin {
		for i := range servers {
			servers[i] = redactServerForViewer(servers[i])
		}
	}

	writeJSON(w, http.StatusOK, servers)
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var input models.ServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	srv := input.ToServer()
	srv.ShowRecentMedia = true
	if err := srv.ValidateForCreate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.CreateServer(srv); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.syncServerToPoller(srv)
	s.InvalidateLibraryCache()
	s.triggerServerSync(srv)
	writeJSON(w, http.StatusCreated, srv)
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := s.store.GetServer(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	user := UserFromContext(r.Context())
	isAdmin := user != nil && user.Role == models.RoleAdmin
	if !isAdmin {
		if srv.DeletedAt != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		redacted := redactServerForViewer(*srv)
		srv = &redacted
	}

	writeJSON(w, http.StatusOK, srv)
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var input models.ServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Always fetch existing server for comparison and preservation
	existing, err := s.store.GetServer(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if existing.DeletedAt != nil {
		writeError(w, http.StatusBadRequest, "cannot update a deleted server")
		return
	}

	// Preserve existing API key and machine_id if not provided in update
	if input.APIKey == "" {
		input.APIKey = existing.APIKey
	}
	if input.MachineID == "" {
		input.MachineID = existing.MachineID
	}

	srv := input.ToServer()
	srv.ID = id
	// Use ValidateForCreate to enforce machine_id for Plex servers on update too
	// This forces remediation of legacy servers when they're edited
	if err := srv.ValidateForCreate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Remove from poller BEFORE DB update to prevent race condition:
	// - Any sync starting now won't find the server in poller (skips)
	// - Any sync that already captured old adapter will fail identity check
	if s.poller != nil {
		s.poller.RemoveServer(id)
	}

	// Atomically update server and clear maintenance data if identity changed
	// Identity = URL + Type + MachineID - any change clears cached data
	if err := s.store.UpdateServerAtomic(existing, srv); err != nil {
		// Restore old adapter on failure
		s.syncServerToPoller(existing)
		writeStoreError(w, err)
		return
	}

	// Add new adapter after DB update succeeds
	s.syncServerToPoller(srv)
	s.InvalidateLibraryCache()
	s.triggerServerSync(srv)
	writeJSON(w, http.StatusOK, srv)
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if r.URL.Query().Get("keep_history") == "true" {
		if err := s.store.SoftDeleteServer(id); err != nil {
			writeStoreError(w, err)
			return
		}
	} else {
		if err := s.store.DeleteServer(id); err != nil {
			writeStoreError(w, err)
			return
		}
	}

	if s.poller != nil {
		s.poller.RemoveServer(id)
	}
	s.InvalidateLibraryCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestoreServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.store.RestoreServer(id); err != nil {
		writeStoreError(w, err)
		return
	}
	srv, err := s.store.GetServer(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.syncServerToPoller(srv)
	s.InvalidateLibraryCache()
	writeJSON(w, http.StatusOK, srv)
}

type testConnectionResult struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	MachineID string `json:"machine_id,omitempty"`
}

func sanitizeConnError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "connection timed out"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS lookup failed"
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return "connection refused or unreachable"
	}
	msg := err.Error()
	if strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") {
		return "authentication failed — check your API key"
	}
	if strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") {
		return "access denied — check your API key"
	}
	return "connection failed"
}

func testConnection(w http.ResponseWriter, r *http.Request, srv models.Server) {
	ms, err := media.NewMediaServer(srv)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// For Plex servers, get identity info including machine_id
	var machineID string
	if srv.Type == models.ServerTypePlex {
		if plexSrv, ok := ms.(*plex.Server); ok {
			identity, err := plexSrv.GetIdentity(r.Context())
			if err != nil {
				log.Printf("test connection for %s failed: %v", srv.Name, err)
				writeJSON(w, http.StatusOK, testConnectionResult{Success: false, Error: sanitizeConnError(err)})
				return
			}
			machineID = identity.MachineIdentifier
		}
	} else {
		if err := ms.TestConnection(r.Context()); err != nil {
			log.Printf("test connection for %s failed: %v", srv.Name, err)
			writeJSON(w, http.StatusOK, testConnectionResult{Success: false, Error: sanitizeConnError(err)})
			return
		}
	}

	writeJSON(w, http.StatusOK, testConnectionResult{Success: true, MachineID: machineID})
}

func (s *Server) triggerServerSync(srv *models.Server) {
	if s.poller == nil {
		return
	}
	if srv.Type == models.ServerTypePlex {
		return
	}
	if !s.autoSync.tryStart(srv.ID) {
		return
	}

	originalURL := srv.URL
	originalType := srv.Type
	originalMachineID := srv.MachineID

	go func() {
		defer s.autoSync.finish(srv.ID)

		ms, ok := s.poller.GetServer(srv.ID)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(s.appCtx, 15*time.Minute)
		defer cancel()

		libs, err := ms.GetLibraries(ctx)
		if err != nil {
			log.Printf("auto-sync: get libraries for server %d: %v", srv.ID, err)
			return
		}

		var synced int
		for _, lib := range libs {
			if ctx.Err() != nil {
				return
			}
			if lib.Type != models.LibraryTypeMovie && lib.Type != models.LibraryTypeShow {
				continue
			}

			items, err := ms.GetLibraryItems(ctx, lib.ID)
			if err != nil {
				log.Printf("auto-sync: fetch items for %s/%s: %v", ms.Name(), lib.Name, err)
				continue
			}

			current, err := s.store.GetServer(srv.ID)
			if err != nil || current.URL != originalURL ||
				current.Type != originalType ||
				current.MachineID != originalMachineID {
				log.Printf("auto-sync: server %d identity changed, aborting", srv.ID)
				return
			}

			count, _, err := s.store.SyncLibraryItems(ctx, srv.ID, lib.ID, items)
			if err != nil {
				log.Printf("auto-sync: save items for %s/%s: %v", ms.Name(), lib.Name, err)
				continue
			}

			synced += count
		}

		if synced > 0 {
			s.InvalidateLibraryCache()
			log.Printf("auto-sync: server %d (%s) — synced %d items across %d libraries",
				srv.ID, ms.Name(), synced, len(libs))
		}
	}()
}

func (s *Server) handleTestServerAdHoc(w http.ResponseWriter, r *http.Request) {
	var input models.ServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	srv := input.ToServer()
	if err := srv.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	testConnection(w, r, *srv)
}

func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := s.store.GetServer(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if srv.DeletedAt != nil {
		writeError(w, http.StatusBadRequest, "cannot test a deleted server")
		return
	}
	testConnection(w, r, *srv)
}
