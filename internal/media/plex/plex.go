package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"streammon/internal/models"
)

type Server struct {
	serverID   int64
	serverName string
	url        string
	token      string
	client     *http.Client
}

func New(srv models.Server) *Server {
	return &Server{
		serverID:   srv.ID,
		serverName: srv.Name,
		url:        strings.TrimRight(srv.URL, "/"),
		token:      srv.APIKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Server) Name() string                { return s.serverName }
func (s *Server) Type() models.ServerType      { return models.ServerTypePlex }

func (s *Server) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/identity", nil)
	if err != nil {
		return err
	}
	s.setHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer drainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex returned status %d", resp.StatusCode)
	}
	return nil
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
	defer drainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return parseSessions(body, s.serverID, s.serverName)
}

func (s *Server) setHeaders(req *http.Request) {
	req.Header.Set("X-Plex-Token", s.token)
	req.Header.Set("Accept", "application/xml")
}

func drainBody(resp *http.Response) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

type mediaContainer struct {
	XMLName xml.Name   `xml:"MediaContainer"`
	Videos  []plexItem `xml:"Video"`
	Tracks  []plexItem `xml:"Track"`
}

type plexItem struct {
	SessionKey       string            `xml:"sessionKey,attr"`
	Type             string            `xml:"type,attr"`
	Title            string            `xml:"title,attr"`
	ParentTitle      string            `xml:"parentTitle,attr"`
	GrandparentTitle string            `xml:"grandparentTitle,attr"`
	Year             string            `xml:"year,attr"`
	Duration         string            `xml:"duration,attr"`
	ViewOffset       string            `xml:"viewOffset,attr"`
	Player           player            `xml:"Player"`
	Session          session           `xml:"Session"`
	User             user              `xml:"User"`
	Media            []plexMedia       `xml:"Media"`
	Thumb            string            `xml:"thumb,attr"`
	TranscodeSession *transcodeSession `xml:"TranscodeSession"`
}

type player struct {
	Title   string `xml:"title,attr"`
	Product string `xml:"product,attr"`
	Address string `xml:"address,attr"`
}

type session struct {
	ID        string `xml:"id,attr"`
	Bandwidth string `xml:"bandwidth,attr"`
}

type user struct {
	Title string `xml:"title,attr"`
}

type plexMedia struct {
	Container       string       `xml:"container,attr"`
	VideoCodec      string       `xml:"videoCodec,attr"`
	AudioCodec      string       `xml:"audioCodec,attr"`
	VideoResolution string       `xml:"videoResolution,attr"`
	Bitrate         string       `xml:"bitrate,attr"`
	AudioChannels   string       `xml:"audioChannels,attr"`
	Parts           []plexPart   `xml:"Part"`
}

type plexPart struct {
	Streams []plexStream `xml:"Stream"`
}

type plexStream struct {
	StreamType string `xml:"streamType,attr"` // 1=video, 2=audio, 3=subtitle
	Codec      string `xml:"codec,attr"`
	Decision   string `xml:"decision,attr"`
}

type transcodeSession struct {
	VideoDecision   string `xml:"videoDecision,attr"`
	AudioDecision   string `xml:"audioDecision,attr"`
	SubtitleDecision string `xml:"subtitleDecision,attr"`
	Progress        string `xml:"progress,attr"`
	Speed           string `xml:"speed,attr"`
	Throttled       string `xml:"throttled,attr"`
	SourceVideoCodec string `xml:"sourceVideoCodec,attr"`
	SourceAudioCodec string `xml:"sourceAudioCodec,attr"`
	VideoCodec      string `xml:"videoCodec,attr"`
	AudioCodec      string `xml:"audioCodec,attr"`
	Container       string `xml:"container,attr"`
	Protocol        string `xml:"protocol,attr"`
	HWRequested     string `xml:"transcodeHwRequested,attr"`
	HWFullPipeline  string `xml:"transcodeHwFullPipeline,attr"`
	HWDecoding      string `xml:"transcodeHwDecoding,attr"`
	HWEncoding      string `xml:"transcodeHwEncoding,attr"`
}

func parseSessions(data []byte, serverID int64, serverName string) ([]models.ActiveStream, error) {
	var mc mediaContainer
	if err := xml.Unmarshal(data, &mc); err != nil {
		return nil, fmt.Errorf("parsing plex XML: %w", err)
	}

	items := make([]plexItem, 0, len(mc.Videos)+len(mc.Tracks))
	items = append(items, mc.Videos...)
	items = append(items, mc.Tracks...)

	streams := make([]models.ActiveStream, 0, len(items))
	for _, item := range items {
		streams = append(streams, buildStream(item, serverID, serverName))
	}
	return streams, nil
}

func buildStream(item plexItem, serverID int64, serverName string) models.ActiveStream {
	as := models.ActiveStream{
		SessionID:        plexSessionID(item),
		ServerID:         serverID,
		ServerName:       serverName,
		ServerType:       models.ServerTypePlex,
		UserName:         item.User.Title,
		MediaType:        plexMediaType(item.Type),
		Title:            item.Title,
		ParentTitle:      item.ParentTitle,
		GrandparentTitle: item.GrandparentTitle,
		Year:             atoi(item.Year),
		DurationMs:       atoi64(item.Duration),
		ProgressMs:       atoi64(item.ViewOffset),
		Player:           item.Player.Title,
		Platform:         item.Player.Product,
		IPAddress:        item.Player.Address,
		Bandwidth:        atoi64(item.Session.Bandwidth) * 1000,  // Plex reports kbps
		StartedAt:        time.Now().UTC(),
	}
	if item.Thumb != "" {
		as.ThumbURL = fmt.Sprintf("/api/servers/%d/thumb/%s", serverID, strings.TrimPrefix(item.Thumb, "/"))
	}
	if len(item.Media) > 0 {
		m := item.Media[0]
		as.VideoCodec = m.VideoCodec
		as.AudioCodec = m.AudioCodec
		as.VideoResolution = normalizeResolution(m.VideoResolution)
		as.Container = m.Container
		as.Bitrate = atoi64(m.Bitrate) * 1000  // Plex reports kbps
		as.AudioChannels = atoi(m.AudioChannels)
		for _, p := range m.Parts {
			for _, st := range p.Streams {
				if st.StreamType == "3" && st.Codec != "" {
					as.SubtitleCodec = st.Codec
				}
			}
		}
	}
	if ts := item.TranscodeSession; ts != nil {
		as.VideoDecision = plexDecision(ts.VideoDecision)
		as.AudioDecision = plexDecision(ts.AudioDecision)
		as.TranscodeHWAccel = ts.HWDecoding == "1" || ts.HWEncoding == "1"
		as.TranscodeProgress = atof(ts.Progress)
		as.TranscodeContainer = ts.Container
		if ts.Protocol != "" {
			as.TranscodeContainer = ts.Protocol
		}
		as.TranscodeVideoCodec = ts.VideoCodec
		as.TranscodeAudioCodec = ts.AudioCodec
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

func plexMediaType(t string) models.MediaType {
	switch t {
	case "movie":
		return models.MediaTypeMovie
	case "episode":
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

func plexSessionID(item plexItem) string {
	if item.Session.ID != "" {
		return item.Session.ID
	}
	return item.SessionKey
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
