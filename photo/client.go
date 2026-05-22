package photo

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
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the photo endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a photo Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// CreateOption customises an outgoing Create request.
type CreateOption func(*createOptions)

type createOptions struct {
	albumId     string
	description string
	date        string
}

// WithAlbum places the new photo in the specified album.
func WithAlbum(albumId string) CreateOption {
	return func(o *createOptions) { o.albumId = albumId }
}

// WithDescription sets the photo's description on upload.
func WithDescription(desc string) CreateOption {
	return func(o *createOptions) { o.description = desc }
}

// WithDate sets the photo's date. Geni accepts a free-form date string
// here (the public docs describe it as "Date in JSON format" without
// specifying); callers should consult Geni's docs for the exact format
// they expect.
func WithDate(date string) CreateOption {
	return func(o *createOptions) { o.date = date }
}

// errInvalidArg is a thin wrapper so client-side argument errors
// produce a consistent message prefix without leaking the helper
// type to callers.
type errInvalidArg string

func (e errInvalidArg) Error() string { return string(e) }

// Create uploads an image to Geni and returns the resulting photo's
// metadata. The endpoint requires multipart/form-data; the client
// builds the body from the supplied io.Reader and filename so callers
// can stream large files without first buffering them as base64 or
// strings.
//
// Both title and a non-nil file are required by Geni; passing an empty
// title or nil file is rejected client-side before the request is sent.
func (c *Client) Create(ctx context.Context, title, fileName string, file io.Reader, opts ...CreateOption) (*Photo, error) {
	if title == "" {
		return nil, errInvalidArg("photo.Create: title is required")
	}
	if file == nil {
		return nil, errInvalidArg("photo.Create: file is required")
	}

	options := createOptions{}
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

	url := c.transport.BaseURL() + "api/photo/add"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	// Pre-set Content-Type with the multipart boundary so the
	// transport's standard-header injection leaves it alone.
	req.Header.Set("Content-Type", w.FormDataContentType())

	respBody, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p Photo
	if err := json.Unmarshal(respBody, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// Get fetches a single photo by id. Concurrent Get calls are coalesced
// into one bulk request via transport.BulkCoalescer.
func (c *Client) Get(ctx context.Context, photoId string) (*Photo, error) {
	url := c.transport.BaseURL() + "api/" + photoId
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[Photo, BulkResponse]{
		CurrentID: photoId,
		IDPrefix:  "photo",
		DecodeBulk: func(body []byte) (BulkResponse, error) {
			var env BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env BulkResponse) []Photo { return env.Results },
		IDOfResult:  func(p Photo) string { return p.ID },
	}

	body, err := c.transport.Do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var p Photo
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// GetBulk fetches multiple photos in a single bulk request. The
// single-id fallback (Geni's bulk dispatcher returns empty for
// len(ids)==1) is preserved verbatim.
func (c *Client) GetBulk(ctx context.Context, photoIds []string) (*BulkResponse, error) {
	if len(photoIds) == 1 {
		one, err := c.Get(ctx, photoIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Photo{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/photo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(photoIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var photos BulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// Update mutates the photo's title / description / date. Body is
// JSON-encoded the same way as profile / document / union update
// endpoints, and runs through transport.EscapeStringToUTF so
// non-ASCII text survives the API's UTF-8 handling.
func (c *Client) Update(ctx context.Context, photoId string, request *Request) (*Photo, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + photoId + "/update"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p Photo
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// Tag associates a profile with a photo. Returns the updated photo
// (its `tags` list will include profileId).
func (c *Client) Tag(ctx context.Context, photoId, profileId string) (*Photo, error) {
	return c.tagAction(ctx, photoId, profileId, "tag")
}

// Untag removes a profile-tag from a photo. Returns the updated photo.
func (c *Client) Untag(ctx context.Context, photoId, profileId string) (*Photo, error) {
	return c.tagAction(ctx, photoId, profileId, "untag")
}

func (c *Client) tagAction(ctx context.Context, photoId, profileId, action string) (*Photo, error) {
	url := c.transport.BaseURL() + "api/" + photoId + "/" + action + "/" + profileId
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p Photo
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// Tags returns the paginated list of profiles tagged in a photo.
// page is 1-indexed; values ≤0 omit the parameter (Geni defaults to
// page 1). Max 50 tags per page.
func (c *Client) Tags(ctx context.Context, photoId string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + photoId + "/tags"
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

// Comments returns the paginated list of comments on a photo. page is
// 1-indexed; values ≤0 omit the parameter (Geni defaults to page 1).
// Max 50 comments per page.
func (c *Client) Comments(ctx context.Context, photoId string, page int) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + photoId + "/comments"
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

	var comments comment.BulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
}

// AddComment posts a new comment on a photo. text is required by Geni;
// title is optional. The response is a [comment.BulkResponse] — the
// updated paginated comment list (sandbox behaviour varies).
func (c *Client) AddComment(ctx context.Context, photoId, text, title string) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + photoId + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
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

	body, err := c.transport.Do(ctx, req, nil)
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

// ForProfile returns the paginated list of photos attached to a
// profile. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 per page.
func (c *Client) ForProfile(ctx context.Context, profileId string, page int) (*BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + profileId + "/photos"
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

	var photos BulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

// AddToProfile attaches a new photo to a profile. Unlike Create
// (which uses multipart/form-data), this endpoint takes a JSON body
// with the file encoded as Base64 in Request.File.
func (c *Client) AddToProfile(ctx context.Context, profileId string, request *Request) (*Photo, error) {
	return c.jsonPost(ctx, profileId, "add-photo", request)
}

// AddMugshotToProfile sets a profile's mugshot — either by uploading
// a new image (MugshotRequest.File, Base64) or by reusing an existing
// photo (MugshotRequest.PhotoId).
func (c *Client) AddMugshotToProfile(ctx context.Context, profileId string, request *MugshotRequest) (*Photo, error) {
	return c.jsonPost(ctx, profileId, "add-mugshot", request)
}

func (c *Client) jsonPost(ctx context.Context, profileId, action string, request any) (*Photo, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + profileId + "/" + action
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p Photo
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &p, nil
}

// Delete deletes a photo by id.
func (c *Client) Delete(ctx context.Context, photoId string) error {
	url := c.transport.BaseURL() + "api/" + photoId + "/delete"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return err
	}

	body, err := c.transport.Do(ctx, req, nil)
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
