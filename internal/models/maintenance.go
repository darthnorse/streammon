package models

import (
	"encoding/json"
	"errors"
	"time"
)

// CriterionType defines the type of maintenance criterion
type CriterionType string

const (
	CriterionUnwatchedMovie  CriterionType = "unwatched_movie"
	CriterionUnwatchedTVNone CriterionType = "unwatched_tv_none"
	CriterionUnwatchedTVLow  CriterionType = "unwatched_tv_low"
	CriterionLowResolution   CriterionType = "low_resolution"
)

// Valid returns true if the criterion type is recognized
func (ct CriterionType) Valid() bool {
	switch ct {
	case CriterionUnwatchedMovie, CriterionUnwatchedTVNone,
		CriterionUnwatchedTVLow, CriterionLowResolution:
		return true
	}
	return false
}

// CriterionTypeInfo describes a criterion type for the API
type CriterionTypeInfo struct {
	Type        CriterionType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	MediaTypes  []MediaType   `json:"media_types"`
	Parameters  []ParamSpec   `json:"parameters"`
}

// ParamSpec describes a parameter for a criterion type
type ParamSpec struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"` // "int", "string"
	Label   string      `json:"label"`
	Default interface{} `json:"default"`
	Min     *int        `json:"min,omitempty"`
	Max     *int        `json:"max,omitempty"`
}

// LibraryItemCache represents a cached library item from a media server
type LibraryItemCache struct {
	ID              int64     `json:"id"`
	ServerID        int64     `json:"server_id"`
	LibraryID       string    `json:"library_id"`
	ItemID          string    `json:"item_id"`
	MediaType       MediaType `json:"media_type"`
	Title           string    `json:"title"`
	Year            int       `json:"year"`
	AddedAt         time.Time `json:"added_at"`
	VideoResolution string    `json:"video_resolution,omitempty"`
	FileSize        int64     `json:"file_size,omitempty"`
	EpisodeCount    int       `json:"episode_count,omitempty"`
	ThumbURL        string    `json:"thumb_url,omitempty"`
	SyncedAt        time.Time `json:"synced_at"`
}

// MaintenanceRule represents a user-defined maintenance rule
type MaintenanceRule struct {
	ID            int64           `json:"id"`
	ServerID      int64           `json:"server_id"`
	LibraryID     string          `json:"library_id"`
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Validate checks that the maintenance rule has valid fields
func (mr *MaintenanceRule) Validate() error {
	if mr.Name == "" {
		return errors.New("name is required")
	}
	if len(mr.Name) > 255 {
		return errors.New("name must be 255 characters or less")
	}
	if mr.ServerID == 0 {
		return errors.New("server_id is required")
	}
	if mr.LibraryID == "" {
		return errors.New("library_id is required")
	}
	if !mr.CriterionType.Valid() {
		return errors.New("invalid criterion type")
	}
	if len(mr.Parameters) == 0 {
		mr.Parameters = json.RawMessage("{}")
	}
	return nil
}

// MaintenanceRuleInput is used for creating/updating rules
type MaintenanceRuleInput struct {
	ServerID      int64           `json:"server_id"`
	LibraryID     string          `json:"library_id"`
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
}

// Validate checks that the input has valid fields
func (in *MaintenanceRuleInput) Validate() error {
	if in.Name == "" {
		return errors.New("name is required")
	}
	if len(in.Name) > 255 {
		return errors.New("name must be 255 characters or less")
	}
	if in.ServerID == 0 {
		return errors.New("server_id is required")
	}
	if in.LibraryID == "" {
		return errors.New("library_id is required")
	}
	if !in.CriterionType.Valid() {
		return errors.New("invalid criterion type")
	}
	if len(in.Parameters) == 0 {
		in.Parameters = json.RawMessage("{}")
	}
	return nil
}

// MaintenanceCandidate represents an item flagged by a rule
type MaintenanceCandidate struct {
	ID            int64             `json:"id"`
	RuleID        int64             `json:"rule_id"`
	LibraryItemID int64             `json:"library_item_id"`
	Reason        string            `json:"reason"`
	ComputedAt    time.Time         `json:"computed_at"`
	Item          *LibraryItemCache `json:"item,omitempty"`
}

// MaintenanceDashboard is the response for the dashboard endpoint
type MaintenanceDashboard struct {
	Libraries []LibraryMaintenance `json:"libraries"`
}

// LibraryMaintenance shows maintenance status for a library
type LibraryMaintenance struct {
	ServerID     int64                      `json:"server_id"`
	ServerName   string                     `json:"server_name"`
	LibraryID    string                     `json:"library_id"`
	LibraryName  string                     `json:"library_name"`
	LibraryType  LibraryType                `json:"library_type"`
	TotalItems   int                        `json:"total_items"`
	LastSyncedAt *time.Time                 `json:"last_synced_at"`
	Rules        []MaintenanceRuleWithCount `json:"rules"`
}

// MaintenanceRuleWithCount includes the candidate count
type MaintenanceRuleWithCount struct {
	MaintenanceRule
	CandidateCount int `json:"candidate_count"`
}

// UnwatchedMovieParams for unwatched_movie criterion
type UnwatchedMovieParams struct {
	Days int `json:"days"`
}

// UnwatchedTVNoneParams for unwatched_tv_none criterion
type UnwatchedTVNoneParams struct {
	Days int `json:"days"`
}

// UnwatchedTVLowParams for unwatched_tv_low criterion
type UnwatchedTVLowParams struct {
	Days       int `json:"days"`
	MaxPercent int `json:"max_percent"`
}

// LowResolutionParams for low_resolution criterion
type LowResolutionParams struct {
	MaxHeight int `json:"max_height"`
}
