package surname

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the surname endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a surname Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get fetches a single surname by id.
func (c *Client) Get(ctx context.Context, surnameId string) (*Surname, error) {
	url := c.transport.BaseURL() + "api/" + surnameId
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
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

// Followers returns the paginated list of profiles following a
// surname. page is 1-indexed; values ≤0 omit the parameter. Max 50
// per page.
func (c *Client) Followers(ctx context.Context, surnameId string, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, surnameId, "followers", page)
}

// Profiles returns the paginated list of profiles associated with a
// surname. page is 1-indexed; values ≤0 omit the parameter. Max 50
// per page.
func (c *Client) Profiles(ctx context.Context, surnameId string, page int) (*profile.BulkResponse, error) {
	return c.profileListing(ctx, surnameId, "profiles", page)
}

func (c *Client) profileListing(ctx context.Context, surnameId, sublist string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + surnameId + "/" + sublist
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
