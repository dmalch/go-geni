package user

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/photoalbum"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/project"
	"github.com/dmalch/go-geni/surname"
	"github.com/dmalch/go-geni/transport"
	"github.com/dmalch/go-geni/video"
)

// Client wraps a transport.Client with the user-scoped endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a user Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get returns the authenticated user — the account behind the OAuth
// token. Use this to ground "me" in API workflows that need to know
// who's calling.
func (c *Client) Get(ctx context.Context) (*User, error) {
	url := c.transport.BaseURL() + "api/user"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &u, nil
}

// FollowedProfiles returns the paginated list of profiles the
// authenticated user follows. page is 1-indexed; values ≤0 omit the
// parameter (Geni defaults to page 1). Max 50 per page.
func (c *Client) FollowedProfiles(ctx context.Context, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, "followed-profiles", page)
}

// FollowedDocuments returns the paginated list of documents the
// authenticated user follows.
func (c *Client) FollowedDocuments(ctx context.Context, page int) (*document.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/followed-documents"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res document.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// UploadedDocuments returns the paginated list of documents the
// authenticated user has uploaded.
func (c *Client) UploadedDocuments(ctx context.Context, page int) (*document.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/uploaded-documents"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res document.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// FollowedProjects returns the paginated list of projects the
// authenticated user follows.
func (c *Client) FollowedProjects(ctx context.Context, page int) (*project.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/followed-projects"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res project.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// FollowedSurnames returns the paginated list of surnames the
// authenticated user follows.
func (c *Client) FollowedSurnames(ctx context.Context, page int) (*surname.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/followed-surnames"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res surname.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// MaxFamily returns the paginated list of profiles in the user's
// "max family" — Geni's term for the set of relatives the user can
// see at maximum depth.
func (c *Client) MaxFamily(ctx context.Context, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, "max-family", page)
}

// UploadedPhotos returns the paginated list of photos the
// authenticated user has uploaded.
func (c *Client) UploadedPhotos(ctx context.Context, page int) (*photo.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/uploaded-photos"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res photo.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// UploadedVideos returns the paginated list of videos the
// authenticated user has uploaded.
func (c *Client) UploadedVideos(ctx context.Context, page int) (*video.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/uploaded-videos"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res video.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// Albums returns the paginated list of the authenticated user's
// photo albums.
func (c *Client) Albums(ctx context.Context, page int) (*photoalbum.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/my-albums"
	body, err := c.getPaginated(ctx, url, page)
	if err != nil {
		return nil, err
	}
	var res photoalbum.BulkResponse
	if err := json.Unmarshal(body, &res); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &res, nil
}

// Labels returns the paginated list of label strings the
// authenticated user has applied to their tree.
func (c *Client) Labels(ctx context.Context, page int) (*LabelsResponse, error) {
	url := c.transport.BaseURL() + "api/user/my-labels"
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

// Metadata returns the authenticated user's application-specific
// metadata (the JSON blob previously stored via UpdateMetadata).
//
// If userIds is non-empty, the call requests metadata for those
// user ids instead of the calling user — Geni's docs describe the
// ids parameter as "Comma separated list of ids of users you would
// like to get metadata for".
func (c *Client) Metadata(ctx context.Context, userIds ...string) (*Metadata, error) {
	url := c.transport.BaseURL() + "api/user/metadata"
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

	body, err := c.transport.Do(ctx, req, nil)
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
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/user/update-metadata"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
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

// profileListing is the shared GET implementation for the
// user-scoped sub-listings that return a profile.BulkResponse
// (followed-profiles, max-family).
func (c *Client) profileListing(ctx context.Context, sublist string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/user/" + sublist
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	profile.AddFields(req)
	if page > 0 {
		query := req.URL.Query()
		query.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = query.Encode()
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var profiles profile.BulkResponse
	if err := json.Unmarshal(body, &profiles); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	apiURL := c.transport.APIURL()
	for i := range profiles.Results {
		profile.StripURLs(&profiles.Results[i], apiURL)
	}
	return &profiles, nil
}

// getPaginated builds a paginated GET request and returns the raw
// response body. Used by the user-scoped listings that decode into
// non-Profile envelopes (document/project/surname/photo/video/
// photoalbum/Labels bulk responses).
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
	return c.transport.Do(ctx, req, nil)
}
