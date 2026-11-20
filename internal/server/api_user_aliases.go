package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/store"
)

type userAliasesResponse struct {
	Aliases           []string `json:"aliases"`
	WatchHistoryMoved int      `json:"watch_history_moved,omitempty"`
}

type linkUserAliasRequest struct {
	Alias string `json:"alias"`
}

func (s *Server) writeUserAliases(w http.ResponseWriter, name string, moved int) {
	aliases, err := s.store.ListUserAliases(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, userAliasesResponse{Aliases: aliases, WatchHistoryMoved: moved})
}

func (s *Server) handleListUserAliases(w http.ResponseWriter, r *http.Request) {
	s.writeUserAliases(w, chi.URLParam(r, "name"), 0)
}

func (s *Server) handleLinkUserAlias(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req linkUserAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if currentUser := UserFromContext(r.Context()); currentUser != nil && currentUser.Name == req.Alias {
		writeError(w, http.StatusBadRequest, "cannot merge yourself into another user")
		return
	}

	result, err := s.store.LinkUserAlias(name, req.Alias)
	if errors.Is(err, store.ErrInvalidAlias) {
		writeError(w, http.StatusBadRequest, "alias must be a different, non-empty username")
		return
	}
	if err != nil {
		writeUserError(w, err)
		return
	}
	s.invalidateOverseerrCaches()
	s.writeUserAliases(w, name, result.WatchHistoryMoved)
}

func (s *Server) handleUnlinkUserAlias(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := s.store.UnlinkUserAlias(name, chi.URLParam(r, "alias"))
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "alias not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.writeUserAliases(w, name, 0)
}
