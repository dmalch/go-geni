package project

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the project endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a project Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get fetches a single project by id.
func (c *Client) Get(ctx context.Context, projectId string) (*Project, error) {
	url := c.transport.BaseURL() + "api/" + projectId
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p Project
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// Profiles returns the paginated list of profiles tagged to a
// project. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 profiles per page.
func (c *Client) Profiles(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, projectId, "profiles", page)
}

// Collaborators returns the paginated list of users who collaborate
// on a project. The response is shaped as a [profile.BulkResponse];
// each entry is a profile object representing the collaborator. page
// is 1-indexed; values ≤0 omit the parameter. Max 50 collaborators
// per page.
func (c *Client) Collaborators(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, projectId, "collaborators", page)
}

// Followers returns the paginated list of users following a project.
// The response is shaped as a [profile.BulkResponse]; each entry is
// a profile object representing the follower. page is 1-indexed;
// values ≤0 omit the parameter. Max 50 followers per page.
func (c *Client) Followers(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, projectId, "followers", page)
}

// AddProfile tags a profile into a project. Returns the updated
// profile (with the new project id in its project_ids list).
func (c *Client) AddProfile(ctx context.Context, profileId, projectId string) (*profile.Profile, error) {
	url := c.transport.BaseURL() + "api/" + projectId + "/add_profiles"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("profile_ids", profileId)
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p profile.Profile
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

func (c *Client) profileListing(ctx context.Context, projectId, sublist string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + projectId + "/" + sublist
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

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
