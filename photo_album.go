package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/photoalbum"
)

// photoAlbumPath maps the album id Geni returns ("album-{n}") to the
// URL-path form Geni's router actually accepts ("photo_album-{n}").
// Bare "album-{n}" requests return a 500 ApiException ("No action
// responded to album-{n}") — the only resource in this client where
// the response id differs from the URL prefix. Callers that already
// pass "photo_album-{n}" are passed through unchanged.
func photoAlbumPath(albumId string) string {
	return "photo_" + strings.TrimPrefix(albumId, "photo_")
}

// CreatePhotoAlbum creates a new photo album for the calling user.
// Returns the newly-created [PhotoAlbum].
func (c *Client) CreatePhotoAlbum(ctx context.Context, request *photoalbum.Request) (*photoalbum.PhotoAlbum, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/photo_album/add"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var album photoalbum.PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}

// GetPhotoAlbum fetches a single photo album by id.
func (c *Client) GetPhotoAlbum(ctx context.Context, albumId string) (*photoalbum.PhotoAlbum, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoAlbumPath(albumId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var album photoalbum.PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}

// GetPhotoAlbumPhotos returns the paginated list of photos in an
// album. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 per page.
func (c *Client) GetPhotoAlbumPhotos(ctx context.Context, albumId string, page int) (*photo.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoAlbumPath(albumId) + "/photos"
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

	var photos photo.BulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// UpdatePhotoAlbum changes a photo album's metadata (name,
// description). Returns the updated [PhotoAlbum].
func (c *Client) UpdatePhotoAlbum(ctx context.Context, albumId string, request *photoalbum.Request) (*photoalbum.PhotoAlbum, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/" + photoAlbumPath(albumId) + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var album photoalbum.PhotoAlbum
	if err := json.Unmarshal(body, &album); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &album, nil
}
