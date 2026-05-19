package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/project"
)

func (c *Client) GetProject(ctx context.Context, projectId string) (*project.Project, error) {
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

	var project project.Project
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
func (c *Client) GetProjectProfiles(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "profiles", page)
}

// GetProjectCollaborators returns the paginated list of users who
// collaborate on a project. The response is shaped as a
// [ProfileBulkResponse]; each entry is a profile object representing
// the collaborator. page is 1-indexed; values ≤0 omit the parameter.
// Max 50 collaborators per page.
func (c *Client) GetProjectCollaborators(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "collaborators", page)
}

// GetProjectFollowers returns the paginated list of users following
// a project. The response is shaped as a [ProfileBulkResponse]; each
// entry is a profile object representing the follower. page is
// 1-indexed; values ≤0 omit the parameter. Max 50 followers per page.
func (c *Client) GetProjectFollowers(ctx context.Context, projectId string, page int) (*profile.BulkResponse, error) {
	return c.getProjectProfileListing(ctx, projectId, "followers", page)
}

// getProjectProfileListing is the shared GET implementation for the
// three /project/<id>/{profiles,collaborators,followers} sub-listings
// — all three return identically-shaped paginated profile envelopes.
func (c *Client) getProjectProfileListing(ctx context.Context, projectId, sublist string, page int) (*profile.BulkResponse, error) {
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

	var profiles profile.BulkResponse
	if err := json.Unmarshal(body, &profiles); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	for i := range profiles.Results {
		c.fixResponse(&profiles.Results[i])
	}
	return &profiles, nil
}

func (c *Client) AddProfileToProject(ctx context.Context, profileId, projectId string) (*profile.Profile, error) {
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

	var profileResponse profile.Profile
	err = json.Unmarshal(body, &profileResponse)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &profileResponse, nil
}

func (c *Client) AddDocumentToProject(ctx context.Context, docimentId, projectId string) (*document.BulkResponse, error) {
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

	var documentResponse document.BulkResponse
	err = json.Unmarshal(body, &documentResponse)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &documentResponse, nil
}
