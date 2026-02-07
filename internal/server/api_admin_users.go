package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

type adminUserResponse struct {
	ID        int64       `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      models.Role `json:"role"`
	ThumbURL  string      `json:"thumb_url"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

func toAdminUserResponse(u *models.User) adminUserResponse {
	return adminUserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		ThumbURL:  u.ThumbURL,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp := make([]adminUserResponse, len(users))
	for i, u := range users {
		resp[i] = toAdminUserResponse(&u)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := s.store.GetUserByID(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

type updateUserRoleRequest struct {
	Role models.Role `json:"role"`
}

func (s *Server) handleAdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
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

	// Prevent demoting the last admin
	if req.Role == models.RoleViewer {
		user, err := s.store.GetUserByID(id)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}

		if user.Role == models.RoleAdmin {
			count, err := s.store.CountAdmins()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal")
				return
			}
			if count <= 1 {
				writeError(w, http.StatusBadRequest, "cannot demote the last admin")
				return
			}
		}
	}

	if err := s.store.UpdateUserRoleByID(id, req.Role); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	user, _ := s.store.GetUserByID(id)
	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Prevent deleting the current user
	currentUser := UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	// Prevent deleting the last admin
	user, err := s.store.GetUserByID(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if user.Role == models.RoleAdmin {
		count, err := s.store.CountAdmins()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}

	if err := s.store.DeleteUser(id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
