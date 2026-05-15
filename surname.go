package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// Surname is Geni's Surname resource — a tag for a family name. Used
// as the parent for surname-scoped collections (followers, profiles).
type Surname struct {
	// Id is the surname's identifier.
	Id string `json:"id,omitempty"`
	// Description is the surname's free-text description.
	Description string `json:"description,omitempty"`
	// SluggedName is the surname rendered as a URL-safe slug (e.g.
	// "smith" for "Smith").
	SluggedName string `json:"slugged_name,omitempty"`
	// Url is the API URL for the surname.
	Url string `json:"url,omitempty"`
}

// GetSurname fetches a single surname by id.
func (c *Client) GetSurname(ctx context.Context, surnameId string) (*Surname, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + surnameId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var s Surname
	if err := json.Unmarshal(body, &s); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &s, nil
}

// GetSurnameFollowers returns the paginated list of profiles
// following a surname. page is 1-indexed; values ≤0 omit the
// parameter. Max 50 per page.
func (c *Client) GetSurnameFollowers(ctx context.Context, surnameId string, page int) (*ProfileBulkResponse, error) {
	return c.getSurnameProfileListing(ctx, surnameId, "followers", page)
}

// GetSurnameProfiles returns the paginated list of profiles
// associated with a surname. page is 1-indexed; values ≤0 omit the
// parameter. Max 50 per page.
func (c *Client) GetSurnameProfiles(ctx context.Context, surnameId string, page int) (*ProfileBulkResponse, error) {
	return c.getSurnameProfileListing(ctx, surnameId, "profiles", page)
}

func (c *Client) getSurnameProfileListing(ctx context.Context, surnameId, sublist string, page int) (*ProfileBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + surnameId + "/" + sublist
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
