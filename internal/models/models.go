package models

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type MediaType string

const (
	MediaTypeMovie      MediaType = "movie"
	MediaTypeTV         MediaType = "episode"
	MediaTypeLiveTV     MediaType = "livetv"
	MediaTypeMusic      MediaType = "track"
	MediaTypeAudiobook  MediaType = "audiobook"
	MediaTypeBook       MediaType = "book"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type ServerType string

const (
	ServerTypePlex    ServerType = "plex"
	ServerTypeEmby    ServerType = "emby"
	ServerTypeJellyfin ServerType = "jellyfin"
)

func (st ServerType) Valid() bool {
	switch st {
	case ServerTypePlex, ServerTypeEmby, ServerTypeJellyfin:
		return true
	}
	return false
}

type Server struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Type      ServerType `json:"type"`
	URL       string     `json:"url"`
	APIKey    string     `json:"-"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *Server) Validate() error {
	if s.Name == "" {
		return errors.New("name is required")
	}
	if !s.Type.Valid() {
		return errors.New("type must be plex, emby, or jellyfin")
	}
	if s.URL == "" {
		return errors.New("url is required")
	}
	if s.APIKey == "" {
		return errors.New("api_key is required")
	}
	return nil
}

type ServerInput struct {
	Name    string     `json:"name"`
	Type    ServerType `json:"type"`
	URL     string     `json:"url"`
	APIKey  string     `json:"api_key"`
	Enabled bool       `json:"enabled"`
}

func (si *ServerInput) ToServer() *Server {
	return &Server{
		Name:    si.Name,
		Type:    si.Type,
		URL:     si.URL,
		APIKey:  si.APIKey,
		Enabled: si.Enabled,
	}
}

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	ThumbURL  string    `json:"thumb_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WatchHistoryEntry struct {
	ID               int64     `json:"id"`
	ServerID         int64     `json:"server_id"`
	UserName         string    `json:"user_name"`
	MediaType        MediaType `json:"media_type"`
	Title            string    `json:"title"`
	ParentTitle      string    `json:"parent_title"`
	GrandparentTitle string    `json:"grandparent_title"`
	Year             int       `json:"year"`
	DurationMs       int64     `json:"duration_ms"`
	WatchedMs        int64     `json:"watched_ms"`
	Player           string    `json:"player"`
	Platform         string    `json:"platform"`
	IPAddress        string    `json:"ip_address"`
	StartedAt        time.Time `json:"started_at"`
	StoppedAt        time.Time `json:"stopped_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type TranscodeDecision string

const (
	TranscodeDecisionDirectPlay TranscodeDecision = "direct play"
	TranscodeDecisionCopy       TranscodeDecision = "copy"
	TranscodeDecisionTranscode  TranscodeDecision = "transcode"
)

type ActiveStream struct {
	SessionID        string    `json:"session_id"`
	ServerID         int64     `json:"server_id"`
	ServerName       string    `json:"server_name"`
	UserName         string    `json:"user_name"`
	MediaType        MediaType `json:"media_type"`
	Title            string    `json:"title"`
	ParentTitle      string    `json:"parent_title"`
	GrandparentTitle string    `json:"grandparent_title"`
	Year             int       `json:"year"`
	DurationMs       int64     `json:"duration_ms"`
	ProgressMs       int64     `json:"progress_ms"`
	Player           string    `json:"player"`
	Platform         string    `json:"platform"`
	IPAddress        string    `json:"ip_address"`
	StartedAt        time.Time `json:"started_at"`

	VideoCodec        string            `json:"video_codec,omitempty"`
	AudioCodec        string            `json:"audio_codec,omitempty"`
	VideoResolution   string            `json:"video_resolution,omitempty"`
	Container         string            `json:"container,omitempty"`
	Bitrate           int64             `json:"bitrate,omitempty"`
	AudioChannels     int               `json:"audio_channels,omitempty"`
	SubtitleCodec     string            `json:"subtitle_codec,omitempty"`
	VideoDecision     TranscodeDecision `json:"video_decision,omitempty"`
	AudioDecision     TranscodeDecision `json:"audio_decision,omitempty"`
	TranscodeHWAccel  bool              `json:"transcode_hw_accel,omitempty"`
	TranscodeProgress float64           `json:"transcode_progress,omitempty"`
	Bandwidth         int64             `json:"bandwidth,omitempty"`
}

type DayStat struct {
	Date       string `json:"date"`
	Movies     int    `json:"movies"`
	TV         int    `json:"tv"`
	LiveTV     int    `json:"livetv"`
	Music      int    `json:"music"`
	Audiobooks int    `json:"audiobooks"`
	Books      int    `json:"books"`
}

type PaginatedResult[T any] struct {
	Items   []T `json:"items"`
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}
