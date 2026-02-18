package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"streammon/internal/models"
	"streammon/internal/store"
)

type StatsResponse struct {
	TopMovies            []models.MediaStat           `json:"top_movies"`
	TopTVShows           []models.MediaStat           `json:"top_tv_shows"`
	TopUsers             []models.UserStat            `json:"top_users"`
	Library              *models.LibraryStat          `json:"library"`
	Locations            []models.GeoResult           `json:"locations"`
	ActivityByDayOfWeek  []models.DayOfWeekStat       `json:"activity_by_day_of_week"`
	ActivityByHour       []models.HourStat            `json:"activity_by_hour"`
	PlatformDistribution []models.DistributionStat    `json:"platform_distribution"`
	PlayerDistribution   []models.DistributionStat    `json:"player_distribution"`
	QualityDistribution  []models.DistributionStat    `json:"quality_distribution"`
	ConcurrentTimeSeries []models.ConcurrentTimePoint `json:"concurrent_time_series"`
	ConcurrentPeaks      models.ConcurrentPeaks       `json:"concurrent_peaks"`
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if viewerName(r) != "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var filter store.StatsFilter

	if d := r.URL.Query().Get("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "days must be a non-negative number")
			return
		}
		filter.Days = parsed
	} else {
		if sd := r.URL.Query().Get("start_date"); sd != "" {
			t, err := time.Parse("2006-01-02", sd)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid start_date, use YYYY-MM-DD")
				return
			}
			filter.StartDate = t
		}
		if ed := r.URL.Query().Get("end_date"); ed != "" {
			t, err := time.Parse("2006-01-02", ed)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid end_date, use YYYY-MM-DD")
				return
			}
			filter.EndDate = t.AddDate(0, 0, 1)
		}
		hasStart := !filter.StartDate.IsZero()
		hasEnd := !filter.EndDate.IsZero()
		if hasStart != hasEnd {
			writeError(w, http.StatusBadRequest, "both start_date and end_date are required")
			return
		}
		if hasStart && hasEnd && !filter.EndDate.After(filter.StartDate) {
			writeError(w, http.StatusBadRequest, "end_date must be after start_date")
			return
		}
	}

	sids, err := parseServerIDs(r.URL.Query().Get("server_ids"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_ids")
		return
	}
	filter.ServerIDs = sids

	var resp StatsResponse
	g, ctx := errgroup.WithContext(r.Context())

	g.Go(func() error {
		var err error
		resp.TopMovies, err = s.store.TopMovies(ctx, 10, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.TopTVShows, err = s.store.TopTVShows(ctx, 10, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.TopUsers, err = s.store.TopUsers(ctx, 10, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.Library, err = s.store.LibraryStats(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.Locations, err = s.store.AllWatchLocations(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ActivityByDayOfWeek, err = s.store.ActivityByDayOfWeek(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ActivityByHour, err = s.store.ActivityByHour(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.PlatformDistribution, err = s.store.PlatformDistribution(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.PlayerDistribution, err = s.store.PlayerDistribution(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.QualityDistribution, err = s.store.QualityDistribution(ctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ConcurrentTimeSeries, resp.ConcurrentPeaks, err = s.store.ConcurrentStats(ctx, filter)
		return err
	})

	if err := g.Wait(); err != nil {
		log.Printf("stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
