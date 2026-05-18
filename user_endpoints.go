package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// PhotoAlbum is Geni's PhotoAlbum resource — a container for related
// photos. Returned by [Client.GetMyAlbums], [Client.CreatePhotoAlbum],
// [Client.GetPhotoAlbum], and [Client.UpdatePhotoAlbum].
type PhotoAlbum struct {
	Id          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	// CoverPhoto is a size-keyed map of cover-image URLs, same
	// shape as PhotoResponse.Sizes.
	CoverPhoto map[string]string `json:"cover_photo,omitempty"`
	// PhotosCount is the number of photos in the album.
	PhotosCount int    `json:"photos_count,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// PhotoAlbumBulkResponse is the paginated envelope returned by
// [Client.GetMyAlbums].
type PhotoAlbumBulkResponse struct {
	Results  []PhotoAlbum `json:"results,omitempty"`
	Page     int          `json:"page,omitempty"`
	NextPage string       `json:"next_page,omitempty"`
	PrevPage string       `json:"prev_page,omitempty"`
}

// LabelsResponse is the paginated envelope returned by
// [Client.GetMyLabels]. Each result is a label string — Geni's docs
// describe my-labels' results as "Array of Strings".
type LabelsResponse struct {
	Results  []string `json:"results,omitempty"`
	Page     int      `json:"page,omitempty"`
	NextPage string   `json:"next_page,omitempty"`
	PrevPage string   `json:"prev_page,omitempty"`
}

// Metadata is Geni's /user/metadata response. Data is the
// application-specific JSON blob the caller previously stored via
// [Client.UpdateMetadata]; the client leaves it as a raw message so
// callers can unmarshal into whatever structure their app uses.
type Metadata struct {
	Data json.RawMessage `json:"data,omitempty"`
}

// GetFollowedProfiles returns the paginated list of profiles the
// authenticated user follows. page is 1-indexed; values ≤0 omit the
// parameter (Geni defaults to page 1). Max 50 per page.
func (c *Client) GetFollowedProfiles(ctx context.Context, page int) (*ProfileBulkResponse, error) {
	return c.getUserProfileListing(ctx, "followed-profiles", page)
}

// GetFollowedDocuments returns the paginated list of documents the
// authenticated user follows.
func (c *Client) GetFollowedDocuments(ctx context.Context, page int) (*DocumentBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/followed-documents"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res DocumentBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetFollowedProjects returns the paginated list of projects the
// authenticated user follows.
func (c *Client) GetFollowedProjects(ctx context.Context, page int) (*ProjectBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/followed-projects"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res ProjectBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetFollowedSurnames returns the paginated list of surnames the
// authenticated user follows.
func (c *Client) GetFollowedSurnames(ctx context.Context, page int) (*SurnameBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/followed-surnames"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res SurnameBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetMaxFamily returns the paginated list of profiles in the user's
// "max family" — Geni's term for the set of relatives the user can
// see at maximum depth.
func (c *Client) GetMaxFamily(ctx context.Context, page int) (*ProfileBulkResponse, error) {
	return c.getUserProfileListing(ctx, "max-family", page)
}

// GetUploadedPhotos returns the paginated list of photos the
// authenticated user has uploaded.
func (c *Client) GetUploadedPhotos(ctx context.Context, page int) (*PhotoBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/uploaded-photos"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res PhotoBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetUploadedVideos returns the paginated list of videos the
// authenticated user has uploaded.
func (c *Client) GetUploadedVideos(ctx context.Context, page int) (*VideoBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/uploaded-videos"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res VideoBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetMyAlbums returns the paginated list of the authenticated user's
// photo albums.
func (c *Client) GetMyAlbums(ctx context.Context, page int) (*PhotoAlbumBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/my-albums"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res PhotoAlbumBulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetMyLabels returns the paginated list of label strings the
// authenticated user has applied to their tree.
func (c *Client) GetMyLabels(ctx context.Context, page int) (*LabelsResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/my-labels"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res LabelsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// GetMetadata returns the authenticated user's application-specific
// metadata (the JSON blob previously stored via UpdateMetadata).
//
// If userIds is non-empty, the call requests metadata for those
// user ids instead of the calling user — Geni's docs describe the
// ids parameter as "Comma separated list of ids of users you would
// like to get metadata for".
func (c *Client) GetMetadata(ctx context.Context, userIds ...string) (*Metadata, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/metadata"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	if len(userIds) > 0 {
		query := req.URL.Query()
		query.Set("ids", strings.Join(userIds, ","))
		req.URL.RawQuery = query.Encode()
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var md Metadata
	if err := json.Unmarshal(body, &md); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &md, nil
}

// UpdateMetadata replaces the authenticated user's application-
// specific metadata with the supplied JSON blob. The data is opaque
// to the client — Geni stores whatever the caller sends.
//
// Wire detail: Geni's /user/update-metadata expects the `data`
// field as a JSON-encoded *string*, not as a nested object (sending
// it as an object returns a 500 "no implicit conversion of
// ActionController::Parameters into String"). The client serialises
// the supplied RawMessage to a string before sending.
func (c *Client) UpdateMetadata(ctx context.Context, data json.RawMessage) (*Metadata, error) {
	payload := map[string]string{"data": string(data)}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/user/update-metadata"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var md Metadata
	if err := json.Unmarshal(body, &md); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &md, nil
}

// getUserProfileListing is the shared GET implementation for the
// user-scoped sub-listings that return a ProfileBulkResponse
// (followed-profiles, max-family). The followed-{documents,projects,
// surnames} and uploaded-{photos,videos} paths each decode a
// different envelope and are inlined above.
func (c *Client) getUserProfileListing(ctx context.Context, sublist string, page int) (*ProfileBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/" + sublist
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)
	if page > 0 {
		query := req.URL.Query()
		query.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = query.Encode()
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profiles ProfileBulkResponse
	if err := json.Unmarshal(body, &profiles); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	for i := range profiles.Results {
		c.fixResponse(&profiles.Results[i])
	}
	return &profiles, nil
}

// getPaginated builds a paginated GET request and returns the raw
// response body. Used by the user-scoped listings that decode into
// non-Profile envelopes (DocumentBulkResponse, ProjectBulkResponse,
// SurnameBulkResponse, PhotoBulkResponse, VideoBulkResponse,
// PhotoAlbumBulkResponse, LabelsResponse).
func (c *Client) getPaginated(ctx context.Context, url string, page int) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	if page > 0 {
		query := req.URL.Query()
		query.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = query.Encode()
	}
	return c.doRequest(ctx, req)
}
