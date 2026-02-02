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
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", c.serverType, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return parseSessions(body, c.serverID, c.serverName)
}

func (c *Client) addAuth(req *http.Request) *http.Request {
	req.Header.Set("X-Emby-Token", c.apiKey)
	return req
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

func parseSessions(data []byte, serverID int64, serverName string) ([]models.ActiveStream, error) {
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
		if len(s.NowPlaying.MediaSources) > 0 {
			src := s.NowPlaying.MediaSources[0]
			as.Container = src.Container
			as.Bitrate = src.Bitrate
			for _, ms := range src.MediaStreams {
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
		}
		if ti := s.TranscodingInfo; ti != nil {
			as.TranscodeProgress = ti.CompletionPct
			as.TranscodeHWAccel = ti.HWAccelerationType != ""
			as.Bandwidth = ti.Bitrate
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
