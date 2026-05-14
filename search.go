package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// SearchProfiles performs a name-based profile search against Geni's
// /profile/search endpoint. The names argument is the free-text query
// Geni matches against profile names (passed to the upstream `names`
// query parameter); pass an empty string to omit it. page is 1-indexed
// and selects which page of results to return — values ≤0 omit the
// parameter (Geni defaults to page 1).
//
// The response is a [ProfileBulkResponse]; its Results, Page,
// NextPage, and PrevPage fields describe the current page and how to
// navigate forward/backward.
func (c *Client) SearchProfiles(ctx context.Context, names string, page int) (*ProfileBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/profile/search"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	query := req.URL.Query()
	if names != "" {
		query.Set("names", names)
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	req.URL.RawQuery = query.Encode()

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
