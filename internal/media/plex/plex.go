package plex

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

type Server struct {
	serverID      int64
	serverName    string
	url           string
	token         string
	client        *http.Client
	metadataCache sync.Map
}

func New(srv models.Server) *Server {
	return &Server{
		serverID:   srv.ID,
		serverName: srv.Name,
		url:        strings.TrimRight(srv.URL, "/"),
		token:      srv.APIKey,
		client:     httputil.NewClient(),
	}
}

func (s *Server) Name() string            { return s.serverName }
func (s *Server) Type() models.ServerType { return models.ServerTypePlex }
func (s *Server) ServerID() int64         { return s.serverID }
func (s *Server) URL() string             { return s.url }

func (s *Server) TestConnection(ctx context.Context) error {
	_, err := s.GetIdentity(ctx)
	return err
}

type IdentityInfo struct {
	MachineIdentifier string
	Version           string
}

func (s *Server) GetIdentity(ctx context.Context) (*IdentityInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/identity", nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httputil.DrainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}

	var ic identityContainer
	if err := xml.Unmarshal(body, &ic); err != nil {
		return nil, fmt.Errorf("parsing identity: %w", err)
	}

	return &IdentityInfo{
		MachineIdentifier: ic.MachineIdentifier,
		Version:           ic.Version,
	}, nil
}

type identityContainer struct {
	XMLName           xml.Name `xml:"MediaContainer"`
	MachineIdentifier string   `xml:"machineIdentifier,attr"`
	Version           string   `xml:"version,attr"`
}

func (s *Server) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/status/sessions", nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httputil.DrainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return s.parseSessions(ctx, body)
}

func (s *Server) getMetadata(ctx context.Context, ratingKey string) *sourceMediaInfo {
	if ratingKey == "" {
		return nil
	}

	if cached, ok := s.metadataCache.Load(ratingKey); ok {
		if info, ok := cached.(*sourceMediaInfo); ok {
			return info
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/library/metadata/"+ratingKey, nil)
	if err != nil {
		slog.Debug("plex: failed to create metadata request", "ratingKey", ratingKey, "error", err)
		return nil
	}
	s.setHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		slog.Debug("plex: failed to fetch metadata", "ratingKey", ratingKey, "error", err)
		return nil
	}
	defer httputil.DrainBody(resp)
	if resp.StatusCode != http.StatusOK {
		slog.Debug("plex: metadata returned non-200", "ratingKey", ratingKey, "status", resp.StatusCode)
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		slog.Debug("plex: failed to read metadata body", "ratingKey", ratingKey, "error", err)
		return nil
	}

	var mc metadataContainer
	if err := xml.Unmarshal(body, &mc); err != nil {
		slog.Debug("plex: failed to parse metadata XML", "ratingKey", ratingKey, "error", err)
		return nil
	}

	var items []metadataItem
	items = append(items, mc.Videos...)
	items = append(items, mc.Tracks...)
	if len(items) == 0 || len(items[0].Media) == 0 {
		slog.Debug("plex: metadata has no media items", "ratingKey", ratingKey)
		return nil
	}

	m := items[0].Media[0]
	res := m.VideoResolution
	if res == "" && m.Height != "" {
		res = heightToResolution(m.Height)
	} else {
		res = normalizeResolution(res)
	}

	info := &sourceMediaInfo{
		VideoCodec:      m.VideoCodec,
		AudioCodec:      m.AudioCodec,
		VideoResolution: res,
		Bitrate:         atoi64(m.Bitrate) * 1000,
		Container:       m.Container,
		AudioChannels:   atoi(m.AudioChannels),
	}

	s.metadataCache.Store(ratingKey, info)
	return info
}

func (s *Server) setHeaders(req *http.Request) {
	req.Header.Set("X-Plex-Token", s.token)
	req.Header.Set("Accept", "application/xml")
}

// Retries up to 2 times on transient 400 errors (e.g. reverse proxy rate limiting).
func (s *Server) DeleteItem(ctx context.Context, itemID string) error {
	const maxRetries = 2
	reqURL := fmt.Sprintf("%s/library/metadata/%s", s.url, url.PathEscape(itemID))

	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			delay := time.Duration(attempt) * time.Second
			slog.Warn("plex: retrying DELETE", "url", reqURL, "attempt", attempt+1, "maxAttempts", maxRetries+1, "delay", delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = s.doDelete(ctx, reqURL)
		if lastErr == nil {
			return nil
		}

		if !errors.Is(lastErr, errPlexBadRequest) {
			return lastErr
		}
		slog.Warn("plex: DELETE returned retryable error", "url", reqURL, "error", lastErr)
	}

	s.client.CloseIdleConnections()
	return fmt.Errorf("plex delete %s: all %d attempts failed: %w", reqURL, maxRetries+1, lastErr)
}

var errPlexBadRequest = errors.New("plex 400")

func (s *Server) doDelete(ctx context.Context, reqURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("plex delete: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode == http.StatusBadRequest {
		slog.Warn("plex: 400 response", "url", reqURL, "body", string(body))
		return errPlexBadRequest
	}
	if len(body) > 0 {
		return fmt.Errorf("plex delete: status %d: %s", resp.StatusCode, body)
	}
	return fmt.Errorf("plex delete: status %d", resp.StatusCode)
}

type mediaContainer struct {
	XMLName xml.Name   `xml:"MediaContainer"`
	Videos  []plexItem `xml:"Video"`
	Tracks  []plexItem `xml:"Track"`
}

type plexItem struct {
	SessionKey            string            `xml:"sessionKey,attr"`
	RatingKey             string            `xml:"ratingKey,attr"`
	GrandparentRatingKey  string            `xml:"grandparentRatingKey,attr"`
	Type                  string            `xml:"type,attr"`
	Title                 string            `xml:"title,attr"`
	ParentTitle           string            `xml:"parentTitle,attr"`
	GrandparentTitle      string            `xml:"grandparentTitle,attr"`
	ParentIndex           string            `xml:"parentIndex,attr"`
	Index                 string            `xml:"index,attr"`
	Year                  string            `xml:"year,attr"`
	Duration              string            `xml:"duration,attr"`
	ViewOffset            string            `xml:"viewOffset,attr"`
	Player                player            `xml:"Player"`
	Session               session           `xml:"Session"`
	User                  user              `xml:"User"`
	Media                 []plexMedia       `xml:"Media"`
	Thumb                 string            `xml:"thumb,attr"`
	GrandparentThumb      string            `xml:"grandparentThumb,attr"`
	TranscodeSession      *transcodeSession `xml:"TranscodeSession"`
}

type player struct {
	Title   string `xml:"title,attr"`
	Product string `xml:"product,attr"`
	Address string `xml:"address,attr"`
	State   string `xml:"state,attr"`
}

type session struct {
	ID        string `xml:"id,attr"`
	Bandwidth string `xml:"bandwidth,attr"`
}

type user struct {
	Title string `xml:"title,attr"`
}

type plexMedia struct {
	ID              string       `xml:"id,attr"`
	Container       string       `xml:"container,attr"`
	VideoCodec      string       `xml:"videoCodec,attr"`
	AudioCodec      string       `xml:"audioCodec,attr"`
	VideoResolution string       `xml:"videoResolution,attr"`
	Height          string       `xml:"height,attr"`
	Width           string       `xml:"width,attr"`
	Bitrate         string       `xml:"bitrate,attr"`
	AudioChannels   string       `xml:"audioChannels,attr"`
	Selected        string       `xml:"selected,attr"`
	Parts           []plexPart   `xml:"Part"`
}

type plexPart struct {
	Streams []plexStream `xml:"Stream"`
}

type plexStream struct {
	StreamType  string `xml:"streamType,attr"` // 1=video, 2=audio, 3=subtitle
	Codec       string `xml:"codec,attr"`
	Decision    string `xml:"decision,attr"`
	Height      string `xml:"height,attr"`
	Width       string `xml:"width,attr"`
	ColorSpace  string `xml:"colorSpace,attr"`
	ColorTrc    string `xml:"colorTrc,attr"`
	DOVIPresent string `xml:"DOVIPresent,attr"`
	DOVIProfile string `xml:"DOVIProfile,attr"`
	BitDepth    string `xml:"bitDepth,attr"`
}

type transcodeSession struct {
	Key              string `xml:"key,attr"`
	VideoDecision    string `xml:"videoDecision,attr"`
	AudioDecision    string `xml:"audioDecision,attr"`
	SubtitleDecision string `xml:"subtitleDecision,attr"`
	Progress         string `xml:"progress,attr"`
	Speed            string `xml:"speed,attr"`
	Throttled        string `xml:"throttled,attr"`
	SourceVideoCodec string `xml:"sourceVideoCodec,attr"`
	SourceAudioCodec string `xml:"sourceAudioCodec,attr"`
	VideoCodec       string `xml:"videoCodec,attr"`
	AudioCodec       string `xml:"audioCodec,attr"`
	VideoResolution  string `xml:"videoResolution,attr"`
	Container        string `xml:"container,attr"`
	Protocol         string `xml:"protocol,attr"`
	Width            string `xml:"width,attr"`
	Height           string `xml:"height,attr"`
	HWRequested      string `xml:"transcodeHwRequested,attr"`
	HWFullPipeline   string `xml:"transcodeHwFullPipeline,attr"`
	HWDecoding       string `xml:"transcodeHwDecoding,attr"`
	HWEncoding       string `xml:"transcodeHwEncoding,attr"`
}

type metadataContainer struct {
	XMLName xml.Name       `xml:"MediaContainer"`
	Videos  []metadataItem `xml:"Video"`
	Tracks  []metadataItem `xml:"Track"`
}

type metadataItem struct {
	RatingKey string          `xml:"ratingKey,attr"`
	Media     []metadataMedia `xml:"Media"`
}

type metadataMedia struct {
	ID              string `xml:"id,attr"`
	VideoCodec      string `xml:"videoCodec,attr"`
	AudioCodec      string `xml:"audioCodec,attr"`
	VideoResolution string `xml:"videoResolution,attr"`
	Height          string `xml:"height,attr"`
	Bitrate         string `xml:"bitrate,attr"`
	AudioChannels   string `xml:"audioChannels,attr"`
	Container       string `xml:"container,attr"`
}

type sourceMediaInfo struct {
	VideoCodec      string
	AudioCodec      string
	VideoResolution string
	Bitrate         int64
	Container       string
	AudioChannels   int
}

func (s *Server) parseSessions(ctx context.Context, data []byte) ([]models.ActiveStream, error) {
	var mc mediaContainer
	if err := xml.Unmarshal(data, &mc); err != nil {
		return nil, fmt.Errorf("parsing plex XML: %w", err)
	}

	items := make([]plexItem, 0, len(mc.Videos)+len(mc.Tracks))
	items = append(items, mc.Videos...)
	items = append(items, mc.Tracks...)

	activeKeys := make(map[string]struct{}, len(items))

	streams := make([]models.ActiveStream, 0, len(items))
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var srcInfo *sourceMediaInfo
		if item.TranscodeSession != nil && item.RatingKey != "" {
			activeKeys[item.RatingKey] = struct{}{}
			srcInfo = s.getMetadata(ctx, item.RatingKey)
		}
		streams = append(streams, buildStream(item, s.serverID, s.serverName, srcInfo))
	}

	s.metadataCache.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok {
			if _, active := activeKeys[k]; !active {
				s.metadataCache.Delete(k)
			}
		}
		return true
	})

	return streams, nil
}

func buildStream(item plexItem, serverID int64, serverName string, srcInfo *sourceMediaInfo) models.ActiveStream {
	as := models.ActiveStream{
		SessionID:         plexSessionID(item),
		ServerID:          serverID,
		ItemID:            item.RatingKey,
		GrandparentItemID: item.GrandparentRatingKey,
		ServerName:        serverName,
		ServerType:        models.ServerTypePlex,
		UserName:          item.User.Title,
		MediaType:         plexMediaType(item.Type),
		Title:             item.Title,
		ParentTitle:       item.ParentTitle,
		GrandparentTitle:  item.GrandparentTitle,
		SeasonNumber:      atoi(item.ParentIndex),
		EpisodeNumber:     atoi(item.Index),
		Year:              atoi(item.Year),
		DurationMs:        atoi64(item.Duration),
		ProgressMs:        atoi64(item.ViewOffset),
		Player:            item.Player.Title,
		Platform:          item.Player.Product,
		IPAddress:         item.Player.Address,
		Bandwidth:         atoi64(item.Session.Bandwidth) * 1000, // Plex reports kbps
		StartedAt:         time.Now().UTC(),
		State:             plexPlayerState(item.Player.State),
	}
	// ThumbURL stores either a rating key (e.g., "55555") or a path fragment
	// (e.g., "library/metadata/12345/thumb/123"). The thumb proxy handler
	// (api_thumb.go) handles both: paths with "/" use the full path, otherwise
	// it constructs /library/metadata/{id}/thumb. For episodes, we prefer the
	// series poster (grandparentRatingKey) over the episode thumbnail.
	if item.GrandparentThumb != "" && item.GrandparentRatingKey != "" {
		as.ThumbURL = item.GrandparentRatingKey
	} else if item.Thumb != "" {
		as.ThumbURL = strings.TrimPrefix(item.Thumb, "/")
	}

	// Media element contains transcoded output during transcoding, not source
	var sessionVideoRes string

	if len(item.Media) > 0 {
		m := item.Media[0]
		for i := range item.Media {
			if item.Media[i].Selected == "1" {
				m = item.Media[i]
				break
			}
		}

		as.VideoCodec = m.VideoCodec
		as.AudioCodec = m.AudioCodec
		if m.VideoResolution != "" {
			sessionVideoRes = normalizeResolution(m.VideoResolution)
		} else if m.Height != "" {
			sessionVideoRes = heightToResolution(m.Height)
		}
		as.VideoResolution = sessionVideoRes
		as.Container = m.Container
		as.Bitrate = atoi64(m.Bitrate) * 1000 // Plex reports kbps
		as.AudioChannels = atoi(m.AudioChannels)

		for _, p := range m.Parts {
			for _, st := range p.Streams {
				if st.StreamType == "1" && as.DynamicRange == "" {
					as.DynamicRange = deriveDynamicRange(st)
				}
				if st.StreamType == "3" && st.Codec != "" {
					as.SubtitleCodec = st.Codec
				}
			}
		}
	}

	if ts := item.TranscodeSession; ts != nil {
		as.TranscodeKey = ts.Key
		as.VideoDecision = plexDecision(ts.VideoDecision)
		as.AudioDecision = plexDecision(ts.AudioDecision)
		as.TranscodeHWDecode = isHWAccel(ts.HWDecoding)
		as.TranscodeHWEncode = isHWAccel(ts.HWEncoding)
		as.TranscodeProgress = atof(ts.Progress)
		as.TranscodeContainer = ts.Container
		if ts.Protocol != "" {
			as.TranscodeContainer = ts.Protocol
		}
		as.TranscodeVideoCodec = ts.VideoCodec
		as.TranscodeAudioCodec = ts.AudioCodec

		if ts.Height != "" {
			as.TranscodeVideoResolution = heightToResolution(ts.Height)
		} else if ts.VideoResolution != "" {
			as.TranscodeVideoResolution = normalizeResolution(ts.VideoResolution)
		} else if sessionVideoRes != "" {
			as.TranscodeVideoResolution = sessionVideoRes
		}

		if srcInfo != nil {
			as.VideoCodec = srcInfo.VideoCodec
			as.AudioCodec = srcInfo.AudioCodec
			as.VideoResolution = srcInfo.VideoResolution
			as.Bitrate = srcInfo.Bitrate
			as.Container = srcInfo.Container
			as.AudioChannels = srcInfo.AudioChannels
		} else if ts.SourceVideoCodec != "" || ts.SourceAudioCodec != "" {
			if ts.SourceVideoCodec != "" {
				as.VideoCodec = ts.SourceVideoCodec
			}
			if ts.SourceAudioCodec != "" {
				as.AudioCodec = ts.SourceAudioCodec
			}
		}
	} else {
		as.VideoDecision = models.TranscodeDecisionDirectPlay
		as.AudioDecision = models.TranscodeDecisionDirectPlay
	}
	return as
}

func plexDecision(d string) models.TranscodeDecision {
	switch d {
	case "transcode":
		return models.TranscodeDecisionTranscode
	case "copy":
		return models.TranscodeDecisionCopy
	default:
		return models.TranscodeDecisionDirectPlay
	}
}

// isHWAccel checks if the transcode HW value indicates hardware acceleration.
// Plex can return "1" (older format) or the codec name like "nvdec", "qsv", "vaapi".
func isHWAccel(val string) bool {
	return val != "" && val != "0"
}

func plexPlayerState(s string) models.SessionState {
	switch s {
	case "paused":
		return models.SessionStatePaused
	case "buffering":
		return models.SessionStateBuffering
	default:
		return models.SessionStatePlaying
	}
}

func plexMediaType(t string) models.MediaType {
	switch t {
	case "movie":
		return models.MediaTypeMovie
	case "episode", "show":
		return models.MediaTypeTV
	case "track":
		return models.MediaTypeMusic
	default:
		return models.MediaType(t)
	}
}

func normalizeResolution(r string) string {
	if r == "" {
		return ""
	}
	if strings.EqualFold(r, "4k") {
		return "4K"
	}
	if _, err := strconv.Atoi(r); err == nil {
		return r + "p"
	}
	return r
}

func heightToResolution(h string) string {
	height := atoi(h)
	return mediautil.HeightToResolution(height)
}

func plexSessionID(item plexItem) string {
	if item.SessionKey != "" {
		return item.SessionKey
	}
	return item.Session.ID
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// deriveDynamicRange determines HDR format from Plex stream color attributes.
// Returns: "Dolby Vision", "Dolby Vision X" (where X is profile), "HDR10", "HLG", "HDR", or "SDR"
func deriveDynamicRange(stream plexStream) string {
	if stream.DOVIPresent == "1" {
		if stream.DOVIProfile != "" {
			return "Dolby Vision " + stream.DOVIProfile
		}
		return "Dolby Vision"
	}

	bitDepth := atoi(stream.BitDepth)
	if stream.ColorSpace == "bt2020" || bitDepth >= 10 {
		switch stream.ColorTrc {
		case "smpte2084":
			return "HDR10"
		case "arib-std-b67":
			return "HLG"
		}
		if stream.ColorSpace == "bt2020" {
			return "HDR"
		}
	}

	return "SDR"
}
