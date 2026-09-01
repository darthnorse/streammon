package overseerr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"streammon/internal/httputil"
)

// ValidateURL checks that the given URL is valid for use as an Overseerr endpoint.
var ValidateURL = httputil.ValidateIntegrationURL

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if err := ValidateURL(baseURL); err != nil {
		return nil, err
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    httputil.NewClientWithTimeout(httputil.IntegrationTimeout),
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) (json.RawMessage, error) {
	return c.doAs(ctx, method, path, query, body, 0)
}

// doAs sets the X-API-User impersonation header when asUserID is non-zero, so
// Overseerr acts as that account rather than the API key's owner.
func (c *Client) doAs(ctx context.Context, method, path string, query url.Values, body io.Reader, asUserID int) (json.RawMessage, error) {
	u := c.baseURL + "/api/v1" + path
	if len(query) > 0 {
		// url.Values.Encode() uses + for spaces per x-www-form-urlencoded;
		// Overseerr expects %20 in query parameters.
		u += "?" + strings.ReplaceAll(query.Encode(), "+", "%20")
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	if asUserID > 0 {
		req.Header.Set("X-API-User", strconv.Itoa(asUserID))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	return readResponse(resp)
}

// StatusError carries the status so callers can tell a rejection the user can
// act on (quota, permission, duplicate) from a broken integration.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("overseerr returned status %d: %s", e.Code, e.Body)
}

// readResponse reads the body from an HTTP response, checks for non-2xx status,
// and returns the raw JSON. The caller must still drain/close the response body.
func readResponse(resp *http.Response) (json.RawMessage, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &StatusError{Code: resp.StatusCode, Body: httputil.Truncate(body, 200)}
	}

	return json.RawMessage(body), nil
}

func (c *Client) doGet(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) doPost(ctx context.Context, path string, payload json.RawMessage) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	return c.do(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) doDelete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

type OverseerrUser struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type pageInfo struct {
	Pages   int `json:"pages"`
	Page    int `json:"page"`
	Results int `json:"results"`
}

type listUsersResponse struct {
	PageInfo pageInfo        `json:"pageInfo"`
	Results  []OverseerrUser `json:"results"`
}

const maxListUsersPages = 100 // safety valve: 5,000 users max

func (c *Client) ListUsers(ctx context.Context) ([]OverseerrUser, error) {
	const pageSize = 50
	var all []OverseerrUser

	for page := 0; page < maxListUsersPages; page++ {
		skip := page * pageSize
		params := url.Values{}
		params.Set("take", strconv.Itoa(pageSize))
		if skip > 0 {
			params.Set("skip", strconv.Itoa(skip))
		}

		raw, err := c.doGet(ctx, "/user", params)
		if err != nil {
			return nil, fmt.Errorf("listing users: %w", err)
		}

		var resp listUsersResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("parsing user list: %w", err)
		}

		if all == nil {
			all = make([]OverseerrUser, 0, resp.PageInfo.Pages*pageSize)
		}
		all = append(all, resp.Results...)

		if resp.PageInfo.Page >= resp.PageInfo.Pages || len(resp.Results) < pageSize {
			break
		}
	}

	return all, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doGet(ctx, "/status", nil)
	return err
}

func (c *Client) Search(ctx context.Context, query string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("query", query)
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	return c.doGet(ctx, "/search", params)
}

func (c *Client) Discover(ctx context.Context, category string, page int) (json.RawMessage, error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	return c.doGet(ctx, "/discover/"+category, params)
}

func (c *Client) GetMovie(ctx context.Context, movieID int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/movie/%d", movieID), nil)
}

func (c *Client) GetTV(ctx context.Context, tvID int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/tv/%d", tvID), nil)
}

func (c *Client) GetTVSeason(ctx context.Context, tvID, seasonNumber int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber), nil)
}

func (c *Client) ListRequests(ctx context.Context, take, skip, requestedBy int, filter, sort string) (json.RawMessage, error) {
	params := url.Values{}
	if take > 0 {
		params.Set("take", strconv.Itoa(take))
	}
	if skip > 0 {
		params.Set("skip", strconv.Itoa(skip))
	}
	if requestedBy > 0 {
		params.Set("requestedBy", strconv.Itoa(requestedBy))
	}
	if filter != "" {
		params.Set("filter", filter)
	}
	if sort != "" {
		params.Set("sort", sort)
	}
	return c.doGet(ctx, "/request", params)
}

func (c *Client) RequestCount(ctx context.Context) (json.RawMessage, error) {
	return c.doGet(ctx, "/request/count", nil)
}

func (c *Client) GetRequest(ctx context.Context, requestID int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/request/%d", requestID), nil)
}

func (c *Client) CreateRequest(ctx context.Context, reqBody json.RawMessage) (json.RawMessage, error) {
	return c.doPost(ctx, "/request", reqBody)
}

// CreateRequestAsUser submits as the given Overseerr user, so their own
// permissions, approval rules and quota apply.
//
// reqBody must not carry a userId. Overseerr honours that field only for
// callers holding MANAGE_USERS/MANAGE_REQUESTS, and it leaves approval judged
// against the API key's owner rather than the requester.
func (c *Client) CreateRequestAsUser(ctx context.Context, userID int, reqBody json.RawMessage) (json.RawMessage, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid overseerr user id %d", userID)
	}
	return c.doAs(ctx, http.MethodPost, "/request", nil, bytes.NewReader(reqBody), userID)
}

func (c *Client) UpdateRequestStatus(ctx context.Context, requestID int, status string) (json.RawMessage, error) {
	if status != "approve" && status != "decline" {
		return nil, fmt.Errorf("invalid status: must be 'approve' or 'decline'")
	}
	return c.doPost(ctx, fmt.Sprintf("/request/%d/%s", requestID, status), nil)
}

func (c *Client) DeleteRequest(ctx context.Context, requestID int) error {
	return c.doDelete(ctx, fmt.Sprintf("/request/%d", requestID))
}

type mediaInfoResponse struct {
	MediaInfo *struct {
		ID       int `json:"id"`
		Requests []struct {
			ID int `json:"id"`
		} `json:"requests"`
	} `json:"mediaInfo"`
}

// MediaLookupResult holds the IDs found when looking up media by TMDB ID.
type MediaLookupResult struct {
	RequestID int // First request ID, or 0 if none
	MediaID   int // Media entry ID, or 0 if none
}

// FindRequestByTMDB looks up the Overseerr media entry for a given TMDB ID and media type.
// Uses the movie/tv detail endpoint for an O(1) lookup instead of scanning all requests.
// mediaType must be "movie" or "tv"; any other value defaults to "movie".
func (c *Client) FindRequestByTMDB(ctx context.Context, tmdbID int, mediaType string) (MediaLookupResult, error) {
	path := fmt.Sprintf("/movie/%d", tmdbID)
	if mediaType == "tv" {
		path = fmt.Sprintf("/tv/%d", tmdbID)
	}

	raw, err := c.doGet(ctx, path, nil)
	if err != nil {
		return MediaLookupResult{}, fmt.Errorf("fetching media info: %w", err)
	}

	var resp mediaInfoResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return MediaLookupResult{}, fmt.Errorf("parsing media info: %w", err)
	}

	var result MediaLookupResult
	if resp.MediaInfo != nil {
		result.MediaID = resp.MediaInfo.ID
		if len(resp.MediaInfo.Requests) > 0 {
			result.RequestID = resp.MediaInfo.Requests[0].ID
		}
	}

	return result, nil
}

type mediaEntry struct {
	TmdbID    int    `json:"tmdbId"`
	MediaType string `json:"mediaType"`
	Status    int    `json:"status"`
}

type listMediaResponse struct {
	PageInfo pageInfo     `json:"pageInfo"`
	Results  []mediaEntry `json:"results"`
}

const maxListMediaPages = 200 // safety valve: 10,000 entries max

// ListMedia returns all Overseerr media entries as a map of "mediaType:tmdbId" → status.
func (c *Client) ListMedia(ctx context.Context) (map[string]int, error) {
	const pageSize = 50
	statuses := make(map[string]int)

	for page := 0; page < maxListMediaPages; page++ {
		skip := page * pageSize
		params := url.Values{}
		params.Set("take", strconv.Itoa(pageSize))
		if skip > 0 {
			params.Set("skip", strconv.Itoa(skip))
		}

		raw, err := c.doGet(ctx, "/media", params)
		if err != nil {
			return nil, fmt.Errorf("listing media: %w", err)
		}

		var resp listMediaResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("parsing media list: %w", err)
		}

		for _, m := range resp.Results {
			if m.TmdbID > 0 && m.Status > 0 && m.MediaType != "" {
				statuses[m.MediaType+":"+strconv.Itoa(m.TmdbID)] = m.Status
			}
		}

		if resp.PageInfo.Page >= resp.PageInfo.Pages || len(resp.Results) < pageSize {
			break
		}
	}

	return statuses, nil
}

func (c *Client) DeleteMedia(ctx context.Context, mediaID int) error {
	return c.doDelete(ctx, fmt.Sprintf("/media/%d", mediaID))
}
