package photoalbum

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the photo-album endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a photoalbum Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// albumPath maps the album id Geni returns ("album-{n}") to the URL
// path Geni's router actually accepts ("photo_album-{n}"). Bare
// "album-{n}" requests return a 500 ApiException ("No action
// responded to album-{n}") — the only resource in this client where
// the response id differs from the URL prefix. Callers that already
// pass "photo_album-{n}" are passed through unchanged.
func albumPath(albumId string) string {
	return "photo_" + strings.TrimPrefix(albumId, "photo_")
}

// Create creates a new photo album for the calling user. Returns the
// newly-created [PhotoAlbum].
func (c *Client) Create(ctx context.Context, request *Request) (*PhotoAlbum, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/photo_album/add"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var album PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}

// Get fetches a single photo album by id.
func (c *Client) Get(ctx context.Context, albumId string) (*PhotoAlbum, error) {
	url := c.transport.BaseURL() + "api/" + albumPath(albumId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var album PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}

// Photos returns the paginated list of photos in an album. page is
// 1-indexed; values ≤0 omit the parameter (Geni defaults to page 1).
// Max 50 per page.
func (c *Client) Photos(ctx context.Context, albumId string, page int) (*photo.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + albumPath(albumId) + "/photos"
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

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var photos photo.BulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// Update changes a photo album's metadata (name, description).
// Returns the updated [PhotoAlbum].
func (c *Client) Update(ctx context.Context, albumId string, request *Request) (*PhotoAlbum, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + albumPath(albumId) + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var album PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}
