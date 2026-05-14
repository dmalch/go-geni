package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
)

// PhotoResponse is Geni's Photo resource — a single uploaded image
// with metadata and tagging.
type PhotoResponse struct {
	// Id is the photo's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the photo's legacy global identifier.
	Guid string `json:"guid,omitempty"`
	// AlbumId is the id of the album containing the photo.
	AlbumId string `json:"album_id,omitempty"`
	// Title is the photo's title.
	Title string `json:"title,omitempty"`
	// Description is the photo's description.
	Description string `json:"description,omitempty"`
	// Date is the photo's date, as returned by Geni (string format).
	Date string `json:"date,omitempty"`
	// Attribution is the photo's attribution string.
	Attribution string `json:"attribution,omitempty"`
	// ContentType is the original MIME type of the upload.
	ContentType string `json:"content_type,omitempty"`
	// Location is the photo's optional location.
	Location *LocationElement `json:"location,omitempty"`
	// Tags is the list of profiles tagged in the photo (urls or ids
	// depending on the `only_ids` query parameter).
	Tags []string `json:"tags,omitempty"`
	// Sizes maps Geni-defined size names (e.g. "small", "medium",
	// "large") to fully-qualified image URLs. The exact set of keys
	// varies by upload.
	Sizes map[string]string `json:"sizes,omitempty"`
	// Url is the API URL for the photo.
	Url string `json:"url,omitempty"`
	// CreatedAt / UpdatedAt are the resource lifecycle timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// PhotoBulkResponse is the envelope returned by [Client.GetPhotos].
type PhotoBulkResponse struct {
	Results []PhotoResponse `json:"results,omitempty"`
}

// CreatePhotoOption customises an outgoing CreatePhoto request.
type CreatePhotoOption func(*createPhotoOptions)

type createPhotoOptions struct {
	albumId     string
	description string
	date        string
}

// WithPhotoAlbum places the new photo in the specified album.
func WithPhotoAlbum(albumId string) CreatePhotoOption {
	return func(o *createPhotoOptions) { o.albumId = albumId }
}

// WithPhotoDescription sets the photo's description on upload.
func WithPhotoDescription(desc string) CreatePhotoOption {
	return func(o *createPhotoOptions) { o.description = desc }
}

// WithPhotoDate sets the photo's date. Geni accepts a free-form date
// string here (the public docs describe it as "Date in JSON format"
// without specifying); callers should consult Geni's docs for the
// exact format they expect.
func WithPhotoDate(date string) CreatePhotoOption {
	return func(o *createPhotoOptions) { o.date = date }
}

// CreatePhoto uploads an image to Geni and returns the resulting
// photo's metadata. The endpoint requires multipart/form-data; the
// client builds the body from the supplied io.Reader and filename so
// callers can stream large files without first buffering them as
// base64 or strings.
//
// Both title and a non-nil file are required by Geni; passing an empty
// title or nil file is rejected client-side before the request is
// sent.
func (c *Client) CreatePhoto(ctx context.Context, title, fileName string, file io.Reader, opts ...CreatePhotoOption) (*PhotoResponse, error) {
	if title == "" {
		return nil, errInvalidArg("CreatePhoto: title is required")
	}
	if file == nil {
		return nil, errInvalidArg("CreatePhoto: file is required")
	}

	options := createPhotoOptions{}
	for _, o := range opts {
		o(&options)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("title", title); err != nil {
		return nil, err
	}
	if options.albumId != "" {
		if err := w.WriteField("album_id", options.albumId); err != nil {
			return nil, err
		}
	}
	if options.description != "" {
		if err := w.WriteField("description", options.description); err != nil {
			return nil, err
		}
	}
	if options.date != "" {
		if err := w.WriteField("date", options.date); err != nil {
			return nil, err
		}
	}
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, file); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	url := BaseURL(c.useSandboxEnv) + "api/photo/add"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	// Pre-set Content-Type with the multipart boundary so the
	// addStandardHeadersAndQueryParams shim leaves it alone.
	req.Header.Set("Content-Type", w.FormDataContentType())

	respBody, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var photo PhotoResponse
	if err := json.Unmarshal(respBody, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// GetPhoto fetches a single photo by id.
func (c *Client) GetPhoto(ctx context.Context, photoId string) (*PhotoResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var photo PhotoResponse
	if err := json.Unmarshal(body, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// GetPhotos fetches multiple photos in a single bulk request.
func (c *Client) GetPhotos(ctx context.Context, photoIds []string) (*PhotoBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/photo"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(photoIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var photos PhotoBulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// DeletePhoto deletes a photo by id.
func (c *Client) DeletePhoto(ctx context.Context, photoId string) error {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/delete"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	var result ResultResponse
	if err := json.Unmarshal(body, &result); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return err
	}
	return nil
}

// errInvalidArg is a thin wrapper so client-side argument errors
// produce a consistent message prefix without leaking the helper
// type to callers.
type errInvalidArg string

func (e errInvalidArg) Error() string { return string(e) }
