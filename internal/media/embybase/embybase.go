package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Name              string            `json:"Name"`
	SeriesName        string            `json:"SeriesName"`
	SeriesId          string            `json:"SeriesId"`
	SeriesPrimaryImageTag string        `json:"SeriesPrimaryImageTag"`
	SeasonName        string            `json:"SeasonName"`
	Type              string            `json:"Type"`
	ProductionYear    int               `json:"ProductionYear"`
	RunTimeTicks      int64             `json:"RunTimeTicks"`
	MediaSources      []mediaSource     `json:"MediaSources"`
	ID                string            `json:"Id"`
	ImageTags         map[string]string `json:"ImageTags"`
	Container         string            `json:"Container"`
	Bitrate           int64             `json:"Bitrate"`
	MediaStreams      []mediaStream     `json:"MediaStreams"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	IndexNumber       int               `json:"IndexNumber"`
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
			SessionID:         s.ID,
			ServerID:          serverID,
			ItemID:            s.NowPlaying.ID,
			GrandparentItemID: s.NowPlaying.SeriesId,
			ServerName:        serverName,
			ServerType:        serverType,
			UserName:          s.UserName,
			MediaType:         embyMediaType(s.NowPlaying.Type),
			Title:             s.NowPlaying.Name,
			ParentTitle:       s.NowPlaying.SeasonName,
			GrandparentTitle:  s.NowPlaying.SeriesName,
			SeasonNumber:      s.NowPlaying.ParentIndexNumber,
			EpisodeNumber:     s.NowPlaying.IndexNumber,
			Year:              s.NowPlaying.ProductionYear,
			DurationMs:        ticksToMs(s.NowPlaying.RunTimeTicks),
			ProgressMs:        ticksToMs(playPos(s.PlayState)),
			Player:            s.DeviceName,
			Platform:          s.Client,
			IPAddress:         s.RemoteIP,
			StartedAt:         time.Now().UTC(),
		}
		if s.NowPlaying.SeriesId != "" && s.NowPlaying.SeriesPrimaryImageTag != "" {
			as.ThumbURL = s.NowPlaying.SeriesId
		} else if s.NowPlaying.ID != "" && s.NowPlaying.ImageTags["Primary"] != "" {
			as.ThumbURL = s.NowPlaying.ID
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
			hwAccel := ti.HWAccelerationType != ""
			as.TranscodeHWDecode = hwAccel
			as.TranscodeHWEncode = hwAccel
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
				as.TranscodeVideoResolution = fmt.Sprintf("%dp", ti.Height)
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
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	ProductionYear    int               `json:"ProductionYear"`
	Type              string            `json:"Type"`
	ImageTags         map[string]string `json:"ImageTags"`
	DateCreated       string            `json:"DateCreated"`
	SeriesName        string            `json:"SeriesName,omitempty"`
	SeriesId          string            `json:"SeriesId,omitempty"`
	SeriesPrimaryImageTag string        `json:"SeriesPrimaryImageTag,omitempty"`
	ParentIndexNumber int               `json:"ParentIndexNumber,omitempty"`
	IndexNumber       int               `json:"IndexNumber,omitempty"`
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
		if item.SeriesId != "" && item.SeriesPrimaryImageTag != "" {
			// For episodes, use series poster instead of episode thumbnail
			thumbURL = item.SeriesId
		} else if item.ImageTags["Primary"] != "" {
			thumbURL = item.ID
		}

		items = append(items, models.LibraryItem{
			ItemID:        item.ID,
			Title:         title,
			Year:          item.ProductionYear,
			MediaType:     embyMediaType(item.Type),
			ThumbURL:      thumbURL,
			AddedAt:       addedAt.UTC(),
			ServerID:      c.serverID,
			ServerName:    c.serverName,
			ServerType:    c.serverType,
			SeasonNumber:  item.ParentIndexNumber,
			EpisodeNumber: item.IndexNumber,
		})
	}

	return items, nil
}

type itemDetailsJSON struct {
	ID                string              `json:"Id"`
	Name              string              `json:"Name"`
	ProductionYear    int                 `json:"ProductionYear"`
	Overview          string              `json:"Overview"`
	Type              string              `json:"Type"`
	ImageTags         map[string]string   `json:"ImageTags"`
	Genres            []string            `json:"Genres"`
	CommunityRating   float64             `json:"CommunityRating"`
	OfficialRating    string              `json:"OfficialRating"`
	RunTimeTicks      int64               `json:"RunTimeTicks"`
	Studios           []studioJSON        `json:"Studios"`
	People            []personJSON        `json:"People"`
	SeriesName        string              `json:"SeriesName"`
	ParentIndexNumber int                 `json:"ParentIndexNumber"`
	IndexNumber       int                 `json:"IndexNumber"`
	MediaSources      []mediaSourceDetail `json:"MediaSources"`
}

type mediaSourceDetail struct {
	Container string               `json:"Container"`
	Bitrate   int64                `json:"Bitrate"`
	Streams   []mediaStreamDetail  `json:"MediaStreams"`
}

type mediaStreamDetail struct {
	Type           string `json:"Type"`
	Codec          string `json:"Codec"`
	Height         int    `json:"Height"`
	Width          int    `json:"Width"`
	Channels       int    `json:"Channels"`
	DisplayTitle   string `json:"DisplayTitle"`
}

type studioJSON struct {
	Name string `json:"Name"`
}

type personJSON struct {
	Name            string `json:"Name"`
	Role            string `json:"Role"`
	Type            string `json:"Type"`
	PrimaryImageTag string `json:"PrimaryImageTag"`
	ID              string `json:"Id"`
}

func (c *Client) GetLibraries(ctx context.Context) ([]models.Library, error) {
	folders, err := c.getVirtualFolders(ctx)
	if err != nil {
		return nil, err
	}

	libraries := make([]models.Library, 0, len(folders))
	for _, folder := range folders {
		lib := models.Library{
			ID:         folder.ItemID,
			ServerID:   c.serverID,
			ServerName: c.serverName,
			ServerType: c.serverType,
			Name:       folder.Name,
			Type:       embyLibraryType(folder.CollectionType),
		}

		counts, err := c.getLibraryCounts(ctx, folder.ItemID, folder.CollectionType)
		if err != nil {
			return nil, fmt.Errorf("getting counts for library %s: %w", folder.Name, err)
		}
		lib.ItemCount = counts.items
		lib.ChildCount = counts.children
		lib.GrandchildCount = counts.grandchildren

		libraries = append(libraries, lib)
	}

	return libraries, nil
}

type virtualFolder struct {
	Name           string `json:"Name"`
	CollectionType string `json:"CollectionType"`
	ItemID         string `json:"ItemId"`
}

func (c *Client) getVirtualFolders(ctx context.Context) ([]virtualFolder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/Library/VirtualFolders", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return nil, fmt.Errorf("%s virtual folders: %w", c.serverType, err)
	}
	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s virtual folders: status %d", c.serverType, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var folders []virtualFolder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("%s parse virtual folders: %w", c.serverType, err)
	}

	return folders, nil
}

type libraryCounts struct {
	items        int
	children     int
	grandchildren int
}

func (c *Client) getLibraryCounts(ctx context.Context, parentID, collectionType string) (*libraryCounts, error) {
	counts := &libraryCounts{}

	switch collectionType {
	case "movies":
		count, err := c.countItems(ctx, parentID, "Movie")
		if err != nil {
			return nil, err
		}
		counts.items = count

	case "tvshows":
		seriesCount, err := c.countItems(ctx, parentID, "Series")
		if err != nil {
			return nil, err
		}
		counts.items = seriesCount

		seasonCount, err := c.countItems(ctx, parentID, "Season")
		if err != nil {
			return nil, err
		}
		counts.children = seasonCount

		episodeCount, err := c.countItems(ctx, parentID, "Episode")
		if err != nil {
			return nil, err
		}
		counts.grandchildren = episodeCount

	case "music":
		artistCount, err := c.countItems(ctx, parentID, "MusicArtist")
		if err != nil {
			return nil, err
		}
		counts.items = artistCount

		albumCount, err := c.countItems(ctx, parentID, "MusicAlbum")
		if err != nil {
			return nil, err
		}
		counts.children = albumCount

		trackCount, err := c.countItems(ctx, parentID, "Audio")
		if err != nil {
			return nil, err
		}
		counts.grandchildren = trackCount

	default:
		count, err := c.countItems(ctx, parentID, "")
		if err != nil {
			return nil, err
		}
		counts.items = count
	}

	return counts, nil
}

type itemCountResponse struct {
	TotalRecordCount int `json:"TotalRecordCount"`
}

func (c *Client) countItems(ctx context.Context, parentID, itemTypes string) (int, error) {
	params := url.Values{
		"ParentId":  {parentID},
		"Recursive": {"true"},
		"Limit":     {"0"},
	}
	if itemTypes != "" {
		params.Set("IncludeItemTypes", itemTypes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/Items?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return 0, fmt.Errorf("%s count items: %w", c.serverType, err)
	}
	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s count items: status %d", c.serverType, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}

	var countResp itemCountResponse
	if err := json.Unmarshal(body, &countResp); err != nil {
		return 0, fmt.Errorf("%s parse count: %w", c.serverType, err)
	}

	return countResp.TotalRecordCount, nil
}

func embyLibraryType(collectionType string) models.LibraryType {
	switch collectionType {
	case "movies":
		return models.LibraryTypeMovie
	case "tvshows":
		return models.LibraryTypeShow
	case "music":
		return models.LibraryTypeMusic
	default:
		return models.LibraryTypeOther
	}
}

func (c *Client) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	// Use Items?Ids= endpoint which doesn't require user context
	url := fmt.Sprintf("%s/Items?Ids=%s&Fields=Overview,Genres,People,Studios,ProductionYear,OfficialRating,CommunityRating,MediaSources", c.url, itemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return nil, fmt.Errorf("%s item details: %w", c.serverType, err)
	}
	defer drainBody(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, models.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s item details: status %d", c.serverType, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	var container struct {
		Items []itemDetailsJSON `json:"Items"`
	}
	if err := json.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("%s parse item details: %w", c.serverType, err)
	}
	if len(container.Items) == 0 {
		return nil, models.ErrNotFound
	}
	item := container.Items[0]

	directors := make([]string, 0)
	cast := make([]models.CastMember, 0)
	for _, p := range item.People {
		switch p.Type {
		case "Director":
			directors = append(directors, p.Name)
		case "Actor":
			// Emby/Jellyfin actor images use just the person ID (e.g., "12345")
			// so we use thumb/%s (with slash) to build the proxy URL
			var thumbURL string
			if p.PrimaryImageTag != "" && p.ID != "" {
				thumbURL = fmt.Sprintf("/api/servers/%d/thumb/%s", c.serverID, p.ID)
			}
			cast = append(cast, models.CastMember{
				Name:     p.Name,
				Role:     p.Role,
				ThumbURL: thumbURL,
			})
		}
	}

	var thumbURL string
	if item.ImageTags["Primary"] != "" {
		thumbURL = fmt.Sprintf("/api/servers/%d/thumb/%s", c.serverID, item.ID)
	}

	var studio string
	if len(item.Studios) > 0 {
		studio = item.Studios[0].Name
	}

	details := &models.ItemDetails{
		ID:            item.ID,
		Title:         item.Name,
		Year:          item.ProductionYear,
		Summary:       item.Overview,
		MediaType:     embyMediaType(item.Type),
		ThumbURL:      thumbURL,
		Genres:        item.Genres,
		Directors:     directors,
		Cast:          cast,
		Rating:        item.CommunityRating,
		ContentRating: item.OfficialRating,
		DurationMs:    ticksToMs(item.RunTimeTicks),
		Studio:        studio,
		SeriesTitle:   item.SeriesName,
		SeasonNumber:  item.ParentIndexNumber,
		EpisodeNumber: item.IndexNumber,
		ServerID:      c.serverID,
		ServerName:    c.serverName,
		ServerType:    c.serverType,
	}

	if len(item.MediaSources) > 0 {
		ms := item.MediaSources[0]
		details.Container = ms.Container
		details.Bitrate = ms.Bitrate
		for _, stream := range ms.Streams {
			switch stream.Type {
			case "Video":
				details.VideoCodec = stream.Codec
				if stream.Height > 0 {
					details.VideoResolution = fmt.Sprintf("%dp", stream.Height)
				}
			case "Audio":
				details.AudioCodec = stream.Codec
				details.AudioChannels = stream.Channels
			}
		}
	}

	return details, nil
}
