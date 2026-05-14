package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

type ProjectBulkResponse struct {
	Results  []ProjectResponse `json:"results,omitempty"`
	Page     int               `json:"page,omitempty"`
	NextPage string            `json:"next_page,omitempty"`
	PrevPage string            `json:"prev_page,omitempty"`
}

type ProjectResponse struct {
	// The project's id
	Id string `json:"id,omitempty"`
	// The project's name
	Name string `json:"name,omitempty"`
	// The project's description
	Description *string `json:"description,omitempty"`
	// UpdatedAt is the timestamp of when the project was last updated
	UpdatedAt string `json:"updated_at,omitempty"`
	// CreatedAt is the timestamp of when the project was created
	CreatedAt string `json:"created_at,omitempty"`
}

func (c *Client) GetProject(ctx context.Context, projectId string) (*ProjectResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + projectId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var project ProjectResponse
	err = json.Unmarshal(body, &project)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &project, nil
}

// GetProjectProfiles returns the paginated list of profiles tagged
// to a project. page is 1-indexed; values ≤0 omit the parameter
// (Geni defaults to page 1). Max 50 profiles per page.
func (c *Client) GetProjectProfiles(ctx context.Context, projectId string, page int) (*ProfileBulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "profiles", page)
}

// GetProjectCollaborators returns the paginated list of users who
// collaborate on a project. The response is shaped as a
// [ProfileBulkResponse]; each entry is a profile object representing
// the collaborator. page is 1-indexed; values ≤0 omit the parameter.
// Max 50 collaborators per page.
func (c *Client) GetProjectCollaborators(ctx context.Context, projectId string, page int) (*ProfileBulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "collaborators", page)
}

// GetProjectFollowers returns the paginated list of users following
// a project. The response is shaped as a [ProfileBulkResponse]; each
// entry is a profile object representing the follower. page is
// 1-indexed; values ≤0 omit the parameter. Max 50 followers per page.
func (c *Client) GetProjectFollowers(ctx context.Context, projectId string, page int) (*ProfileBulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "followers", page)
}

// getProjectProfileListing is the shared GET implementation for the
// three /project/<id>/{profiles,collaborators,followers} sub-listings
// — all three return identically-shaped paginated profile envelopes.
func (c *Client) getProjectProfileListing(ctx context.Context, projectId, sublist string, page int) (*ProfileBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + projectId + "/" + sublist
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

func (c *Client) AddProfileToProject(ctx context.Context, profileId, projectId string) (*ProfileResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + projectId + "/add_profiles"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("profile_ids", profileId)
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profileResponse ProfileResponse
	err = json.Unmarshal(body, &profileResponse)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &profileResponse, nil
}

func (c *Client) AddDocumentToProject(ctx context.Context, docimentId, projectId string) (*DocumentBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + projectId + "/add_documents"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", docimentId)
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var documentResponse DocumentBulkResponse
	err = json.Unmarshal(body, &documentResponse)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &documentResponse, nil
}
