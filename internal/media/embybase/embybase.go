package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"streammon/internal/models"
)

type Client struct {
	serverID   int64
	serverName string
	serverType models.ServerType
	url        string
	apiKey     string
	client     *http.Client
}

func New(srv models.Server, serverType models.ServerType) *Client {
	return &Client{
		serverID:   srv.ID,
		serverName: srv.Name,
		serverType: serverType,
		url:        strings.TrimRight(srv.URL, "/"),
		apiKey:     srv.APIKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Name() string              { return c.serverName }
func (c *Client) Type() models.ServerType    { return c.serverType }

func (c *Client) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/System/Info/Public", nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return err
	}
	defer drainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", c.serverType, resp.StatusCode)
	}
	return nil
}

func (c *Client) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/Sessions", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return nil, err
	}
	defer drainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", c.serverType, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return parseSessions(body, c.serverID, c.serverName, c.serverType)
}

func (c *Client) addAuth(req *http.Request) *http.Request {
	req.Header.Set("X-Emby-Token", c.apiKey)
	return req
}

func drainBody(resp *http.Response) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

type embySession struct {
	ID       string    `json:"Id"`
	UserName string    `json:"UserName"`
	Client   string    `json:"Client"`
	DeviceName string    `json:"DeviceName"`
	RemoteIP string    `json:"RemoteEndPoint"`
	NowPlaying     *nowPlaying     `json:"NowPlayingItem"`
	PlayState      *playState      `json:"PlayState"`
	TranscodingInfo *transcodingInfo `json:"TranscodingInfo"`
}

type nowPlaying struct {
	Name           string         `json:"Name"`
	SeriesName     string         `json:"SeriesName"`
	SeasonName     string         `json:"SeasonName"`
	Type           string         `json:"Type"`
	ProductionYear int            `json:"ProductionYear"`
	RunTimeTicks   int64          `json:"RunTimeTicks"`
	MediaSources   []mediaSource  `json:"MediaSources"`
	ID             string            `json:"Id"`
	ImageTags      map[string]string `json:"ImageTags"`
	Container      string          `json:"Container"`
	Bitrate        int64           `json:"Bitrate"`
	MediaStreams    []mediaStream   `json:"MediaStreams"`
}

type mediaSource struct {
	Container    string        `json:"Container"`
	Bitrate      int64         `json:"Bitrate"`
	MediaStreams  []mediaStream `json:"MediaStreams"`
}

type mediaStream struct {
	Type         string `json:"Type"` // Video, Audio, Subtitle
	Codec        string `json:"Codec"`
	Channels     int    `json:"Channels"`
	Height       int    `json:"Height"`
	DisplayTitle string `json:"DisplayTitle"`
}

type playState struct {
	PositionTicks int64 `json:"PositionTicks"`
}

type transcodingInfo struct {
	IsVideoDirect       bool    `json:"IsVideoDirect"`
	IsAudioDirect       bool    `json:"IsAudioDirect"`
	Container           string  `json:"Container"`
	VideoCodec          string  `json:"VideoCodec"`
	AudioCodec          string  `json:"AudioCodec"`
	Bitrate             int64   `json:"Bitrate"`
	CompletionPct       float64 `json:"CompletionPercentage"`
	Width               int     `json:"Width"`
	Height              int     `json:"Height"`
	AudioChannels       int     `json:"AudioChannels"`
	HWAccelerationType  string  `json:"HardwareAccelerationType"`
	TranscodeReasons    []string `json:"TranscodeReasons"`
}

func parseSessions(data []byte, serverID int64, serverName string, serverType models.ServerType) ([]models.ActiveStream, error) {
	var sessions []embySession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions JSON: %w", err)
	}

	var streams []models.ActiveStream
	for _, s := range sessions {
		if s.NowPlaying == nil {
			continue
		}
		as := models.ActiveStream{
			SessionID:        s.ID,
			ServerID:         serverID,
			ServerName:       serverName,
			ServerType:       serverType,
			UserName:         s.UserName,
			MediaType:        embyMediaType(s.NowPlaying.Type),
			Title:            s.NowPlaying.Name,
			ParentTitle:      s.NowPlaying.SeasonName,
			GrandparentTitle: s.NowPlaying.SeriesName,
			Year:             s.NowPlaying.ProductionYear,
			DurationMs:       ticksToMs(s.NowPlaying.RunTimeTicks),
			ProgressMs:       ticksToMs(playPos(s.PlayState)),
			Player:           s.DeviceName,
			Platform:         s.Client,
			IPAddress:        s.RemoteIP,
			StartedAt:        time.Now().UTC(),
		}
		if s.NowPlaying.ID != "" && s.NowPlaying.ImageTags["Primary"] != "" {
			as.ThumbURL = fmt.Sprintf("/api/servers/%d/thumb/%s", serverID, s.NowPlaying.ID)
		}
		var container string
		var bitrate int64
		var mediaStreams []mediaStream
		if len(s.NowPlaying.MediaSources) > 0 {
			src := s.NowPlaying.MediaSources[0]
			container = src.Container
			bitrate = src.Bitrate
			mediaStreams = src.MediaStreams
		} else {
			container = s.NowPlaying.Container
			bitrate = s.NowPlaying.Bitrate
			mediaStreams = s.NowPlaying.MediaStreams
		}
		as.Container = container
		as.Bitrate = bitrate
		for _, ms := range mediaStreams {
			switch ms.Type {
			case "Video":
				as.VideoCodec = ms.Codec
				if ms.Height > 0 {
					as.VideoResolution = fmt.Sprintf("%dp", ms.Height)
				}
			case "Audio":
				as.AudioCodec = ms.Codec
				as.AudioChannels = ms.Channels
			case "Subtitle":
				as.SubtitleCodec = ms.Codec
			}
		}
		if ti := s.TranscodingInfo; ti != nil {
			as.TranscodeProgress = ti.CompletionPct
			as.TranscodeHWAccel = ti.HWAccelerationType != ""
			as.Bandwidth = ti.Bitrate
			as.TranscodeContainer = ti.Container
			as.TranscodeVideoCodec = ti.VideoCodec
			as.TranscodeAudioCodec = ti.AudioCodec
			if ti.IsVideoDirect {
				as.VideoDecision = models.TranscodeDecisionDirectPlay
			} else {
				as.VideoDecision = models.TranscodeDecisionTranscode
			}
			if ti.IsAudioDirect {
				as.AudioDecision = models.TranscodeDecisionDirectPlay
			} else {
				as.AudioDecision = models.TranscodeDecisionTranscode
			}
			if ti.Height > 0 {
				as.VideoResolution = fmt.Sprintf("%dp", ti.Height)
			}
		} else {
			as.VideoDecision = models.TranscodeDecisionDirectPlay
			as.AudioDecision = models.TranscodeDecisionDirectPlay
		}
		streams = append(streams, as)
	}
	return streams, nil
}

func playPos(ps *playState) int64 {
	if ps == nil {
		return 0
	}
	return ps.PositionTicks
}

func ticksToMs(ticks int64) int64 {
	return ticks / 10000
}

func embyMediaType(t string) models.MediaType {
	switch t {
	case "Movie", "MusicVideo", "Video":
		return models.MediaTypeMovie
	case "Episode":
		return models.MediaTypeTV
	case "Audio":
		return models.MediaTypeMusic
	case "TvChannel":
		return models.MediaTypeLiveTV
	case "AudioBook":
		return models.MediaTypeAudiobook
	case "Book":
		return models.MediaTypeBook
	default:
		return models.MediaType(strings.ToLower(t))
	}
}

type libraryItemsResponse struct {
	Items []libraryItemJSON `json:"Items"`
}

type libraryItemJSON struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	ProductionYear int               `json:"ProductionYear"`
	Type           string            `json:"Type"`
	ImageTags      map[string]string `json:"ImageTags"`
	DateCreated    string            `json:"DateCreated"`
	SeriesName     string            `json:"SeriesName,omitempty"`
}

func (c *Client) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	url := fmt.Sprintf("%s/Items?Recursive=true&SortBy=DateCreated&SortOrder=Descending&Limit=%d&IncludeItemTypes=Movie,Episode",
		c.url, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return nil, fmt.Errorf("%s recently added: %w", c.serverType, err)
	}
	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s recently added: status %d", c.serverType, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	var data libraryItemsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("%s parse recently added: %w", c.serverType, err)
	}

	items := make([]models.LibraryItem, 0, len(data.Items))
	for _, item := range data.Items {
		addedAt, err := time.Parse(time.RFC3339, item.DateCreated)
		if err != nil {
			addedAt = time.Now().UTC()
		}

		title := item.Name
		if item.SeriesName != "" {
			title = item.SeriesName + " - " + item.Name
		}

		var thumbURL string
		if item.ImageTags["Primary"] != "" {
			thumbURL = item.ID
		}

		items = append(items, models.LibraryItem{
			Title:      title,
			Year:       item.ProductionYear,
			MediaType:  embyMediaType(item.Type),
			ThumbURL:   thumbURL,
			AddedAt:    addedAt.UTC(),
			ServerID:   c.serverID,
			ServerName: c.serverName,
			ServerType: c.serverType,
		})
	}

	return items, nil
}
