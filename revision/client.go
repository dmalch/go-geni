package revision

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the revision endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a revision Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get fetches a single revision by id.
func (c *Client) Get(ctx context.Context, revisionId string) (*Revision, error) {
	url := c.transport.BaseURL() + "api/" + revisionId
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var r Revision
	if err := json.Unmarshal(body, &r); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &r, nil
}

// GetBulk fetches multiple revisions in one call. Mirrors the
// single-id bulk fallback used by other bulk Get methods — when
// len(ids) == 1 the request goes through the singular Get path so
// Geni's bulk dispatcher quirk doesn't return empty results.
func (c *Client) GetBulk(ctx context.Context, revisionIds []string) (*BulkResponse, error) {
	if len(revisionIds) == 1 {
		one, err := c.Get(ctx, revisionIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Revision{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/revision"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(revisionIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var revisions BulkResponse
	if err := json.Unmarshal(body, &revisions); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &revisions, nil
}
