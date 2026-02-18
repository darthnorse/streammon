package server

import (
	"log"
	"net/http"
	"strconv"

	"golang.org/x/sync/errgroup"

	"streammon/internal/models"
)

var allowedFilterDays = map[int]bool{0: true, 7: true, 30: true}

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
	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleViewer {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	days := 0
	if d := r.URL.Query().Get("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil {
			writeError(w, http.StatusBadRequest, "days must be a number")
			return
		}
		if !allowedFilterDays[parsed] {
			writeError(w, http.StatusBadRequest, "days must be 0, 7, or 30")
			return
		}
		days = parsed
	}

	var resp StatsResponse
	g, ctx := errgroup.WithContext(r.Context())

	g.Go(func() error {
		var err error
		resp.TopMovies, err = s.store.TopMovies(ctx, 10, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.TopTVShows, err = s.store.TopTVShows(ctx, 10, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.TopUsers, err = s.store.TopUsers(ctx, 10, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.Library, err = s.store.LibraryStats(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.Locations, err = s.store.AllWatchLocations(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ActivityByDayOfWeek, err = s.store.ActivityByDayOfWeek(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ActivityByHour, err = s.store.ActivityByHour(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.PlatformDistribution, err = s.store.PlatformDistribution(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.PlayerDistribution, err = s.store.PlayerDistribution(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.QualityDistribution, err = s.store.QualityDistribution(ctx, days)
		return err
	})
	g.Go(func() error {
		var err error
		resp.ConcurrentTimeSeries, resp.ConcurrentPeaks, err = s.store.ConcurrentStats(ctx, days)
		return err
	})

	if err := g.Wait(); err != nil {
		log.Printf("stats error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
