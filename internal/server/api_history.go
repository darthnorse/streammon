package server

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
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
	if vn := viewerName(r); vn != "" {
		userFilter = vn
	}

	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	sortColumn := ""
	if col, ok := allowedSortColumns[sortBy]; ok {
		sortColumn = col
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	serverIDs, err := parseServerIDs(r.URL.Query().Get("server_ids"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_ids")
		return
	}

	result, err := s.store.ListHistory(page, perPage, userFilter, sortColumn, sortOrder, serverIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if s.geoResolver != nil {
		for i := range result.Items {
			e := &result.Items[i]
			if e.IPAddress != "" && e.City == "" && e.Country == "" {
				ip := net.ParseIP(e.IPAddress)
				if ip == nil {
					continue
				}
				geo := s.geoResolver.Lookup(ip)
				if geo == nil {
					continue
				}
				e.City = geo.City
				e.Country = geo.Country
				e.ISP = geo.ISP
				if err := s.store.SetCachedGeo(geo); err != nil {
					log.Printf("auto-cache geo for %s: %v", e.IPAddress, err)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if vn := viewerName(r); vn != "" {
		owner, err := s.store.GetHistoryOwner(id)
		if err != nil || owner != vn {
			writeJSON(w, http.StatusOK, []models.WatchSession{})
			return
		}
	}
	sessions, err := s.store.ListSessionsForHistory(id)
	if err != nil {
		log.Printf("listing sessions for history %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if sessions == nil {
		sessions = []models.WatchSession{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleDailyHistory(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start date, use YYYY-MM-DD")
		return
	}
	endParsed, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end date, use YYYY-MM-DD")
		return
	}
	// Make end date exclusive (include the full end day)
	end := endParsed.AddDate(0, 0, 1)
	if !end.After(start) {
		writeError(w, http.StatusBadRequest, "end must not be before start")
		return
	}

	userFilter := viewerName(r)

	serverIDs, err := parseServerIDs(r.URL.Query().Get("server_ids"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_ids")
		return
	}

	stats, err := s.store.DailyWatchCountsForUser(start, end, userFilter, serverIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
