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
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Type            ServerType `json:"type"`
	URL             string     `json:"url"`
	APIKey          string     `json:"-"`
	Enabled         bool       `json:"enabled"`
	ShowRecentMedia bool       `json:"show_recent_media"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	Name            string     `json:"name"`
	Type            ServerType `json:"type"`
	URL             string     `json:"url"`
	APIKey          string     `json:"api_key"`
	Enabled         bool       `json:"enabled"`
	ShowRecentMedia bool       `json:"show_recent_media"`
}

func (si *ServerInput) ToServer() *Server {
	return &Server{
		Name:            si.Name,
		Type:            si.Type,
		URL:             si.URL,
		APIKey:          si.APIKey,
		Enabled:         si.Enabled,
		ShowRecentMedia: si.ShowRecentMedia,
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
	SeasonNumber     int       `json:"season_number,omitempty"`
	EpisodeNumber    int       `json:"episode_number,omitempty"`
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
	ServerName       string     `json:"server_name"`
	ServerType       ServerType `json:"server_type"`
	UserName         string     `json:"user_name"`
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
	LastPollSeen     time.Time `json:"-"`

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
	ThumbURL            string            `json:"thumb_url,omitempty"`
	TranscodeContainer  string            `json:"transcode_container,omitempty"`
	TranscodeVideoCodec string            `json:"transcode_video_codec,omitempty"`
	TranscodeAudioCodec string            `json:"transcode_audio_codec,omitempty"`
	SeasonNumber        int               `json:"season_number,omitempty"`
	EpisodeNumber       int               `json:"episode_number,omitempty"`
}

type SessionState string

const (
	SessionStatePlaying   SessionState = "playing"
	SessionStatePaused    SessionState = "paused"
	SessionStateStopped   SessionState = "stopped"
	SessionStateBuffering SessionState = "buffering"
)

type SessionUpdate struct {
	SessionKey string
	RatingKey  string
	State      SessionState
	ViewOffset int64 // progress in milliseconds
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

type GeoResult struct {
	IP       string  `json:"ip"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	City     string  `json:"city"`
	Country  string  `json:"country"`
	LastSeen *string `json:"last_seen,omitempty"`
}

type LibraryItem struct {
	ItemID        string     `json:"item_id"`
	Title         string     `json:"title"`
	Year          int        `json:"year,omitempty"`
	MediaType     MediaType  `json:"media_type"`
	ThumbURL      string     `json:"thumb_url,omitempty"`
	AddedAt       time.Time  `json:"added_at"`
	ServerID      int64      `json:"server_id"`
	ServerName    string     `json:"server_name"`
	ServerType    ServerType `json:"server_type"`
	SeasonNumber  int        `json:"season_number,omitempty"`
	EpisodeNumber int        `json:"episode_number,omitempty"`
}

type CastMember struct {
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`
}

type ItemDetails struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	Year          int          `json:"year,omitempty"`
	Summary       string       `json:"summary,omitempty"`
	MediaType     MediaType    `json:"media_type"`
	ThumbURL      string       `json:"thumb_url,omitempty"`
	Genres        []string     `json:"genres,omitempty"`
	Directors     []string     `json:"directors,omitempty"`
	Cast          []CastMember `json:"cast,omitempty"`
	Rating        float64      `json:"rating,omitempty"`
	ContentRating string       `json:"content_rating,omitempty"`
	DurationMs    int64        `json:"duration_ms,omitempty"`
	Studio        string       `json:"studio,omitempty"`
	SeriesTitle   string       `json:"series_title,omitempty"`
	SeasonNumber  int          `json:"season_number,omitempty"`
	EpisodeNumber int          `json:"episode_number,omitempty"`
	ServerID      int64        `json:"server_id"`
	ServerName    string       `json:"server_name"`
	ServerType    ServerType   `json:"server_type"`

	VideoResolution string `json:"video_resolution,omitempty"`
	VideoCodec      string `json:"video_codec,omitempty"`
	AudioCodec      string `json:"audio_codec,omitempty"`
	AudioChannels   int    `json:"audio_channels,omitempty"`
	Container       string `json:"container,omitempty"`
	Bitrate         int64  `json:"bitrate,omitempty"`
}

type MediaStat struct {
	Title      string  `json:"title"`
	Year       int     `json:"year,omitempty"`
	PlayCount  int     `json:"play_count"`
	TotalHours float64 `json:"total_hours"`
}

type UserStat struct {
	UserName   string  `json:"user_name"`
	PlayCount  int     `json:"play_count"`
	TotalHours float64 `json:"total_hours"`
}

type LibraryStat struct {
	TotalPlays    int     `json:"total_plays"`
	TotalHours    float64 `json:"total_hours"`
	UniqueUsers   int     `json:"unique_users"`
	UniqueMovies  int     `json:"unique_movies"`
	UniqueTVShows int     `json:"unique_tv_shows"`
}

type SharerAlert struct {
	UserName  string   `json:"user_name"`
	UniqueIPs int      `json:"unique_ips"`
	Locations []string `json:"locations"`
	LastSeen  string   `json:"last_seen"`
}
