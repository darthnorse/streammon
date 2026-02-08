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

	"github.com/go-chi/chi/v5"

	"streammon/internal/media"
	"streammon/internal/media/plex"
	"streammon/internal/models"
)

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
	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	// Redact infrastructure details for non-admin users
	if user := UserFromContext(r.Context()); user == nil || user.Role != models.RoleAdmin {
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

	// Redact infrastructure details for non-admin users
	if user := UserFromContext(r.Context()); user == nil || user.Role != models.RoleAdmin {
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
	writeJSON(w, http.StatusOK, srv)
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := parseServerID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.store.DeleteServer(id); err != nil {
		writeStoreError(w, err)
		return
	}
	if s.poller != nil {
		s.poller.RemoveServer(id)
	}
	s.InvalidateLibraryCache()
	w.WriteHeader(http.StatusNoContent)
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
	testConnection(w, r, *srv)
}
