package rules

import (
	"time"

	"streammon/internal/models"
)

// baseHistoryQuerier provides default nil implementations for all HistoryQuerier methods.
// Embed this in test-specific mocks to avoid boilerplate.
type baseHistoryQuerier struct{}

func (baseHistoryQuerier) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (baseHistoryQuerier) GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	return nil, nil
}

func (baseHistoryQuerier) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	return false, nil
}

func (baseHistoryQuerier) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	return nil, nil
}

func (baseHistoryQuerier) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	return nil, nil
}

func (baseHistoryQuerier) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	return nil, nil
}
