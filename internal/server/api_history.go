package server

import (
	"net/http"
	"strconv"
	"time"
)

const maxPerPage = 100

var allowedSortColumns = map[string]string{
	"started_at": "h.started_at",
	"user":       "h.user_name",
	"title":      "h.title",
	"type":       "h.media_type",
	"duration":   "h.duration_ms",
	"watched":    "h.watched_ms",
	"player":     "h.player",
	"platform":   "h.platform",
	"location":   "g.city",
}

func (s *Server) handleListHistory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	userFilter := r.URL.Query().Get("user")

	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	sortColumn := ""
	if col, ok := allowedSortColumns[sortBy]; ok {
		sortColumn = col
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	result, err := s.store.ListHistory(page, perPage, userFilter, sortColumn, sortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDailyHistory(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start date, use YYYY-MM-DD")
		return
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end date, use YYYY-MM-DD")
		return
	}
	if end.Before(start) {
		writeError(w, http.StatusBadRequest, "end must not be before start")
		return
	}

	stats, err := s.store.DailyWatchCounts(start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
