package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/media"
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

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
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
	if err := srv.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.CreateServer(srv); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.syncServerToPoller(srv)
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
	srv := input.ToServer()
	srv.ID = id
	if err := srv.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.UpdateServer(srv); err != nil {
		writeStoreError(w, err)
		return
	}
	s.syncServerToPoller(srv)
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
	w.WriteHeader(http.StatusNoContent)
}

type testConnectionResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
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
	ms, err := media.NewMediaServer(*srv)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ms.TestConnection(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, testConnectionResult{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, testConnectionResult{Success: true})
}
