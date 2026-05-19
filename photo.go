package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/comment"
	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

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
func (c *Client) CreatePhoto(ctx context.Context, title, fileName string, file io.Reader, opts ...CreatePhotoOption) (*photo.Photo, error) {
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

	var photo photo.Photo
	if err := json.Unmarshal(respBody, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// GetPhoto fetches a single photo by id.
func (c *Client) GetPhoto(ctx context.Context, photoId string) (*photo.Photo, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[photo.Photo, photo.BulkResponse]{
		CurrentID: photoId,
		IDPrefix:  "photo",
		DecodeBulk: func(body []byte) (photo.BulkResponse, error) {
			var env photo.BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env photo.BulkResponse) []photo.Photo { return env.Results },
		IDOfResult:  func(p photo.Photo) string { return p.Id },
	}

	body, err := c.doRequest(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var photo photo.Photo
	if err := json.Unmarshal(body, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// GetPhotos fetches multiple photos in a single bulk request.
func (c *Client) GetPhotos(ctx context.Context, photoIds []string) (*photo.BulkResponse, error) {
	// Single-id fallback — see GetUnions for the Geni-side quirk.
	if len(photoIds) == 1 {
		one, err := c.GetPhoto(ctx, photoIds[0])
		if err != nil {
			return nil, err
		}
		return &photo.BulkResponse{Results: []photo.Photo{*one}}, nil
	}

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

	var photos photo.BulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// UpdatePhoto mutates the photo's title / description / date. Body is
// JSON-encoded the same way as profile / document / union update
// endpoints, and runs through escapeStringToUTF so non-ASCII text
// survives the API's UTF-8 handling.
func (c *Client) UpdatePhoto(ctx context.Context, photoId string, request *photo.Request) (*photo.Photo, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var photo photo.Photo
	if err := json.Unmarshal(body, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// TagPhoto associates a profile with a photo. Returns the updated
// photo (its `tags` list will include profileId).
func (c *Client) TagPhoto(ctx context.Context, photoId, profileId string) (*photo.Photo, error) {
	return c.photoTagAction(ctx, photoId, profileId, "tag")
}

// UntagPhoto removes a profile-tag from a photo. Returns the updated
// photo.
func (c *Client) UntagPhoto(ctx context.Context, photoId, profileId string) (*photo.Photo, error) {
	return c.photoTagAction(ctx, photoId, profileId, "untag")
}

func (c *Client) photoTagAction(ctx context.Context, photoId, profileId, action string) (*photo.Photo, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/" + action + "/" + profileId
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var photo photo.Photo
	if err := json.Unmarshal(body, &photo); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photo, nil
}

// GetPhotoTags returns the paginated list of profiles tagged in a
// photo. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 tags per page.
func (c *Client) GetPhotoTags(ctx context.Context, photoId string, page int) (*profile.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/tags"
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

// GetPhotoComments returns the paginated list of comments on a photo.
// page is 1-indexed; values ≤0 omit the parameter (Geni defaults to
// page 1). Max 50 comments per page.
func (c *Client) GetPhotoComments(ctx context.Context, photoId string, page int) (*comment.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/comments"
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

	var comments comment.BulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
}

// AddPhotoComment posts a new comment on a photo. text is required by
// Geni; title is optional. The response is a [CommentBulkResponse] —
// the updated paginated comment list (sandbox behaviour varies: see
// the analogous note on AddDocumentComment).
func (c *Client) AddPhotoComment(ctx context.Context, photoId, text, title string) (*comment.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + photoId + "/comment"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Set("text", text)
	if title != "" {
		query.Set("title", title)
	}
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var comments comment.BulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
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

	var result transport.Result
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
