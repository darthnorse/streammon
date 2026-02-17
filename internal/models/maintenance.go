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
	CriterionLowResolution   CriterionType = "low_resolution"
	CriterionLargeFiles      CriterionType = "large_files"
)

// Valid returns true if the criterion type is recognized
func (ct CriterionType) Valid() bool {
	switch ct {
	case CriterionUnwatchedMovie, CriterionUnwatchedTVNone,
		CriterionLowResolution, CriterionLargeFiles:
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
	LastWatchedAt   *time.Time `json:"last_watched_at,omitempty"`
	ThumbURL        string     `json:"thumb_url,omitempty"`
	TMDBID          string     `json:"tmdb_id,omitempty"`
	TVDBID          string     `json:"tvdb_id,omitempty"`
	IMDBID          string     `json:"imdb_id,omitempty"`
	SyncedAt        time.Time  `json:"synced_at"`
}

// RuleLibrary represents a server/library association for a maintenance rule
type RuleLibrary struct {
	ServerID  int64  `json:"server_id"`
	LibraryID string `json:"library_id"`
}

// MaintenanceRule represents a user-defined maintenance rule
type MaintenanceRule struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	MediaType     MediaType       `json:"media_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	Libraries     []RuleLibrary   `json:"libraries"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Validate checks that the maintenance rule has valid fields
func (mr *MaintenanceRule) Validate() error {
	if err := validateRuleFields(mr.Name, mr.CriterionType); err != nil {
		return err
	}
	if mr.MediaType != MediaTypeMovie && mr.MediaType != MediaTypeTV {
		return errors.New("media_type must be movie or episode")
	}
	if len(mr.Libraries) == 0 {
		return errors.New("at least one library is required")
	}
	if len(mr.Parameters) == 0 {
		mr.Parameters = json.RawMessage("{}")
	}
	return nil
}

// MaintenanceRuleInput is used for creating rules
type MaintenanceRuleInput struct {
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	MediaType     MediaType       `json:"media_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	Libraries     []RuleLibrary   `json:"libraries"`
}

// Validate checks that the input has valid fields
func (in *MaintenanceRuleInput) Validate() error {
	if err := validateRuleFields(in.Name, in.CriterionType); err != nil {
		return err
	}
	if in.MediaType != MediaTypeMovie && in.MediaType != MediaTypeTV {
		return errors.New("media_type must be movie or episode")
	}
	if len(in.Libraries) == 0 {
		return errors.New("at least one library is required")
	}
	for _, lib := range in.Libraries {
		if lib.ServerID <= 0 || lib.LibraryID == "" {
			return errors.New("each library must have a valid server_id and library_id")
		}
	}
	if len(in.Parameters) == 0 {
		in.Parameters = json.RawMessage("{}")
	}
	return nil
}

// MaintenanceRuleUpdateInput is used for updating rules
type MaintenanceRuleUpdateInput struct {
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	Libraries     []RuleLibrary   `json:"libraries"`
}

// Validate checks that the update input has valid fields
func (in *MaintenanceRuleUpdateInput) Validate() error {
	if err := validateRuleFields(in.Name, in.CriterionType); err != nil {
		return err
	}
	if len(in.Libraries) == 0 {
		return errors.New("at least one library is required")
	}
	for _, lib := range in.Libraries {
		if lib.ServerID <= 0 || lib.LibraryID == "" {
			return errors.New("each library must have a valid server_id and library_id")
		}
	}
	if len(in.Parameters) == 0 {
		in.Parameters = json.RawMessage("{}")
	}
	return nil
}

// validateRuleFields validates common rule fields (DRY helper)
func validateRuleFields(name string, criterionType CriterionType) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 255 {
		return errors.New("name must be 255 characters or less")
	}
	if !criterionType.Valid() {
		return errors.New("invalid criterion type")
	}
	return nil
}

// MaintenanceCandidate represents an item flagged by a rule
type MaintenanceCandidate struct {
	ID               int64             `json:"id"`
	RuleID           int64             `json:"rule_id"`
	LibraryItemID    int64             `json:"library_item_id"`
	Reason           string            `json:"reason"`
	ComputedAt       time.Time         `json:"computed_at"`
	Item             *LibraryItemCache `json:"item,omitempty"`
	CrossServerCount int               `json:"cross_server_count"`
}

// BatchCandidate is used for batch upsert operations
type BatchCandidate struct {
	LibraryItemID int64
	Reason        string
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

type MaintenanceRuleWithCount struct {
	MaintenanceRule
	CandidateCount int `json:"candidate_count"`
	ExclusionCount int `json:"exclusion_count"`
}

// LowResolutionParams for low_resolution criterion
type LowResolutionParams struct {
	MaxHeight int `json:"max_height"`
}

// LargeFilesParams for large_files criterion
type LargeFilesParams struct {
	MinSizeGB float64 `json:"min_size_gb"`
}

// BulkDeleteResult represents the result of a bulk delete operation
type BulkDeleteResult struct {
	Deleted   int               `json:"deleted"`
	Failed    int               `json:"failed"`
	Skipped   int               `json:"skipped"` // Items skipped because they were excluded since page load
	TotalSize int64             `json:"total_size"`
	Errors    []BulkDeleteError `json:"errors,omitempty"`
}

// BulkDeleteError represents a single deletion failure
type BulkDeleteError struct {
	CandidateID int64  `json:"candidate_id"`
	Title       string `json:"title"`
	Error       string `json:"error"`
}

// MaintenanceExclusion represents an excluded item for a rule
type MaintenanceExclusion struct {
	ID            int64             `json:"id"`
	RuleID        int64             `json:"rule_id"`
	LibraryItemID int64             `json:"library_item_id"`
	ExcludedBy    string            `json:"excluded_by"`
	ExcludedAt    time.Time         `json:"excluded_at"`
	Item          *LibraryItemCache `json:"item,omitempty"`
}

// CandidatesResponse is the response for listing candidates with summary stats
type CandidatesResponse struct {
	Items          []MaintenanceCandidate `json:"items"`
	Total          int                    `json:"total"`
	TotalSize      int64                  `json:"total_size"`
	ExclusionCount int                    `json:"exclusion_count"`
	Page           int                    `json:"page"`
	PerPage        int                    `json:"per_page"`
}
