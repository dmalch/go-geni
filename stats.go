package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// StatsResponse is Geni's /stats response — an opaque list of
// "available statistics". The public docs describe the response as an
// array of hashes without enumerating fields, so each entry is kept
// as a raw JSON message; callers decode the specific stats they care
// about.
type StatsResponse struct {
	Stats []json.RawMessage `json:"stats,omitempty"`
}

// GetStats fetches the platform-wide statistics list. The shape of
// individual entries is opaque to the client — see
// [StatsResponse.Stats].
func (c *Client) GetStats(ctx context.Context) (*StatsResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/stats"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var s StatsResponse
	if err := json.Unmarshal(body, &s); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &s, nil
}
