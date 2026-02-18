package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type CriterionType string

const (
	CriterionUnwatchedMovie  CriterionType = "unwatched_movie"
	CriterionUnwatchedTVNone CriterionType = "unwatched_tv_none"
	CriterionLowResolution   CriterionType = "low_resolution"
	CriterionLargeFiles      CriterionType = "large_files"
)

func (ct CriterionType) Valid() bool {
	switch ct {
	case CriterionUnwatchedMovie, CriterionUnwatchedTVNone,
		CriterionLowResolution, CriterionLargeFiles:
		return true
	}
	return false
}

type CriterionTypeInfo struct {
	Type        CriterionType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	MediaTypes  []MediaType   `json:"media_types"`
	Parameters  []ParamSpec   `json:"parameters"`
}

type ParamSpec struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"` // "int", "string"
	Label   string      `json:"label"`
	Default interface{} `json:"default"`
	Min     *int        `json:"min,omitempty"`
	Max     *int        `json:"max,omitempty"`
}

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

type RuleLibrary struct {
	ServerID  int64  `json:"server_id"`
	LibraryID string `json:"library_id"`
}

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

type MaintenanceRuleInput struct {
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	MediaType     MediaType       `json:"media_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	Libraries     []RuleLibrary   `json:"libraries"`
}

func (in *MaintenanceRuleInput) Validate() error {
	if err := validateRuleFields(in.Name, in.CriterionType); err != nil {
		return err
	}
	if in.MediaType != MediaTypeMovie && in.MediaType != MediaTypeTV {
		return errors.New("media_type must be movie or episode")
	}
	if err := validateLibraries(in.Libraries); err != nil {
		return err
	}
	if len(in.Parameters) == 0 {
		in.Parameters = json.RawMessage("{}")
	}
	return nil
}

type MaintenanceRuleUpdateInput struct {
	Name          string          `json:"name"`
	CriterionType CriterionType   `json:"criterion_type"`
	Parameters    json.RawMessage `json:"parameters"`
	Enabled       bool            `json:"enabled"`
	Libraries     []RuleLibrary   `json:"libraries"`
}

func (in *MaintenanceRuleUpdateInput) Validate() error {
	if err := validateRuleFields(in.Name, in.CriterionType); err != nil {
		return err
	}
	if err := validateLibraries(in.Libraries); err != nil {
		return err
	}
	if len(in.Parameters) == 0 {
		in.Parameters = json.RawMessage("{}")
	}
	return nil
}
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

func validateLibraries(libs []RuleLibrary) error {
	if len(libs) == 0 {
		return errors.New("at least one library is required")
	}
	seen := make(map[string]bool, len(libs))
	for _, lib := range libs {
		if lib.ServerID <= 0 || lib.LibraryID == "" {
			return errors.New("each library must have a valid server_id and library_id")
		}
		key := fmt.Sprintf("%d:%s", lib.ServerID, lib.LibraryID)
		if seen[key] {
			return errors.New("duplicate library in request")
		}
		seen[key] = true
	}
	return nil
}

type MaintenanceCandidate struct {
	ID            int64             `json:"id"`
	RuleID        int64             `json:"rule_id"`
	LibraryItemID int64             `json:"library_item_id"`
	Reason        string            `json:"reason"`
	ComputedAt    time.Time         `json:"computed_at"`
	Item          *LibraryItemCache `json:"item,omitempty"`
	OtherCopies   []RuleLibrary     `json:"other_copies,omitempty"`
}

type BatchCandidate struct {
	LibraryItemID int64
	Reason        string
}

type MaintenanceDashboard struct {
	Libraries []LibraryMaintenance `json:"libraries"`
}

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

type LowResolutionParams struct {
	MaxHeight int `json:"max_height"`
}

type LargeFilesParams struct {
	MinSizeGB float64 `json:"min_size_gb"`
}

type BulkDeleteResult struct {
	Deleted   int               `json:"deleted"`
	Failed    int               `json:"failed"`
	Skipped   int               `json:"skipped"` // Items skipped because they were excluded since page load
	TotalSize int64             `json:"total_size"`
	Errors    []BulkDeleteError `json:"errors"`
}

type BulkDeleteError struct {
	CandidateID int64  `json:"candidate_id"`
	Title       string `json:"title"`
	Error       string `json:"error"`
}

type MaintenanceExclusion struct {
	ID            int64             `json:"id"`
	RuleID        int64             `json:"rule_id"`
	LibraryItemID int64             `json:"library_item_id"`
	ExcludedBy    string            `json:"excluded_by"`
	ExcludedAt    time.Time         `json:"excluded_at"`
	Item          *LibraryItemCache `json:"item,omitempty"`
}

type CandidatesResponse struct {
	Items          []MaintenanceCandidate `json:"items"`
	Total          int                    `json:"total"`
	TotalSize      int64                  `json:"total_size"`
	ExclusionCount int                    `json:"exclusion_count"`
	Page           int                    `json:"page"`
	PerPage        int                    `json:"per_page"`
}
