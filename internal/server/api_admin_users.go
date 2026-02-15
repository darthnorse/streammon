package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/store"
)

type adminUserResponse struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	Role       models.Role `json:"role"`
	ThumbURL   string      `json:"thumb_url"`
	Provider   string      `json:"provider"`
	ProviderID string      `json:"provider_id"`
	CreatedAt  string      `json:"created_at"`
	UpdatedAt  string      `json:"updated_at"`
}

func toAdminUserResponse(u *store.AdminUser) adminUserResponse {
	return adminUserResponse{
		ID:         u.ID,
		Name:       u.Name,
		Email:      u.Email,
		Role:       u.Role,
		ThumbURL:   u.ThumbURL,
		Provider:   u.Provider,
		ProviderID: u.ProviderID,
		CreatedAt:  u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func parseUserID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, store.ErrLastAdmin):
		writeError(w, http.StatusBadRequest, "cannot remove the last admin")
	case errors.Is(err, store.ErrNoPassword):
		writeError(w, http.StatusBadRequest, "cannot unlink user without password (would be locked out)")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListAdminUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp := make([]adminUserResponse, len(users))
	for i := range users {
		resp[i] = toAdminUserResponse(&users[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := s.store.GetAdminUserByID(id)
	if err != nil {
		writeUserError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

type updateUserRoleRequest struct {
	Role models.Role `json:"role"`
}

func (s *Server) handleAdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req updateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != models.RoleAdmin && req.Role != models.RoleViewer {
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	if err := s.store.UpdateUserRoleByIDSafe(id, req.Role); err != nil {
		writeUserError(w, err)
		return
	}

	user, err := s.store.GetAdminUserByID(id)
	if err != nil {
		writeUserError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if currentUser := UserFromContext(r.Context()); currentUser != nil && currentUser.ID == id {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := s.store.DeleteUser(id); err != nil {
		writeUserError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminUnlinkUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if currentUser := UserFromContext(r.Context()); currentUser != nil && currentUser.ID == id {
		writeError(w, http.StatusBadRequest, "cannot unlink your own account")
		return
	}

	if err := s.store.UnlinkUserProvider(id); err != nil {
		writeUserError(w, err)
		return
	}

	user, err := s.store.GetAdminUserByID(id)
	if err != nil {
		writeUserError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

type mergeUsersRequest struct {
	KeepID   int64 `json:"keep_id"`
	DeleteID int64 `json:"delete_id"`
}

func (s *Server) handleAdminMergeUsers(w http.ResponseWriter, r *http.Request) {
	var req mergeUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.KeepID == 0 || req.DeleteID == 0 {
		writeError(w, http.StatusBadRequest, "keep_id and delete_id are required")
		return
	}

	if req.KeepID == req.DeleteID {
		writeError(w, http.StatusBadRequest, "cannot merge user with itself")
		return
	}

	if currentUser := UserFromContext(r.Context()); currentUser != nil && currentUser.ID == req.DeleteID {
		writeError(w, http.StatusBadRequest, "cannot merge yourself into another user")
		return
	}

	result, err := s.store.MergeUsers(req.KeepID, req.DeleteID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	s.invalidateOverseerrUserCache()

	writeJSON(w, http.StatusOK, result)
}
