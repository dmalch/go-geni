package video

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

// Client wraps a transport.Client with the video endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a video Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// CreateOption customises an outgoing Create request.
type CreateOption func(*createOptions)

type createOptions struct {
	description string
	date        string
}

// WithDescription sets the video's description on upload.
func WithDescription(desc string) CreateOption {
	return func(o *createOptions) { o.description = desc }
}

// WithDate sets the video's date. Geni accepts a free-form date string
// here (the public docs describe it as "Date in JSON form" without
// specifying); callers should consult Geni's docs for the exact
// format they expect.
func WithDate(date string) CreateOption {
	return func(o *createOptions) { o.date = date }
}

// errInvalidArg is a thin wrapper so client-side argument errors
// produce a consistent message prefix without leaking the helper
// type to callers.
type errInvalidArg string

func (e errInvalidArg) Error() string { return string(e) }

// Create uploads a new video to Geni. The endpoint expects
// multipart/form-data; the client builds the body from the supplied
// io.Reader and filename so callers can stream large files.
//
// Sandbox observation (contradicting the public docs, which list
// `file` as optional): /video/add rejects requests without a file
// part with `400 {"message":"key not found: file"}`, and the server
// runs the uploaded bytes through ffmpeg to validate the format —
// arbitrary byte payloads get rejected with a 500 ApiException
// ("Could not get the duration"). In practice you need a real
// encoded video file. `title` is required by Geni and is enforced
// client-side; `file` and `fileName` may be nil/empty but expect
// the server to reject the call.
func (c *Client) Create(ctx context.Context, title, fileName string, file io.Reader, opts ...CreateOption) (*Video, error) {
	if title == "" {
		return nil, errInvalidArg("video.Create: title is required")
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
	if file != nil {
		fw, err := w.CreateFormFile("file", fileName)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(fw, file); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	url := c.transport.BaseURL() + "api/video/add"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	respBody, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(respBody, &v); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &v, nil
}

// Get fetches a single video by id. Concurrent Get calls are coalesced
// into one bulk request via transport.BulkCoalescer.
func (c *Client) Get(ctx context.Context, videoId string) (*Video, error) {
	url := c.transport.BaseURL() + "api/" + videoId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[Video, BulkResponse]{
		CurrentID: videoId,
		IDPrefix:  "video",
		DecodeBulk: func(body []byte) (BulkResponse, error) {
			var env BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env BulkResponse) []Video { return env.Results },
		IDOfResult:  func(v Video) string { return v.ID },
	}

	body, err := c.transport.Do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(body, &v); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &v, nil
}

// GetBulk fetches multiple videos in a single bulk request. The
// single-id fallback (Geni's bulk dispatcher returns empty for
// len(ids)==1) is preserved verbatim.
func (c *Client) GetBulk(ctx context.Context, videoIds []string) (*BulkResponse, error) {
	if len(videoIds) == 1 {
		one, err := c.Get(ctx, videoIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Video{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/video"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(videoIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var videos BulkResponse
	if err := json.Unmarshal(body, &videos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &videos, nil
}

// Update mutates the video's title / description / date. Body is
// JSON-encoded and run through transport.EscapeStringToUTF for UTF-8
// safety, matching the other update endpoints in the package.
func (c *Client) Update(ctx context.Context, videoId string, request *Request) (*Video, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + videoId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(body, &v); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &v, nil
}

// Delete deletes a video by id.
func (c *Client) Delete(ctx context.Context, videoId string) error {
	url := c.transport.BaseURL() + "api/" + videoId + "/delete"
	req, err := http.NewRequest(http.MethodPost, url, nil)
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

// AddToProfile attaches a new video to a profile. Unlike Create
// (which uses multipart/form-data), this endpoint takes a JSON body
// with the file encoded as Base64 in Request.File.
func (c *Client) AddToProfile(ctx context.Context, profileId string, request *Request) (*Video, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + profileId + "/add-video"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(body, &v); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &v, nil
}

// Tag associates a profile with a video.
func (c *Client) Tag(ctx context.Context, videoId, profileId string) (*Video, error) {
	return c.tagAction(ctx, videoId, profileId, "tag")
}

// Untag removes a profile-tag from a video.
func (c *Client) Untag(ctx context.Context, videoId, profileId string) (*Video, error) {
	return c.tagAction(ctx, videoId, profileId, "untag")
}

func (c *Client) tagAction(ctx context.Context, videoId, profileId, action string) (*Video, error) {
	url := c.transport.BaseURL() + "api/" + videoId + "/" + action + "/" + profileId
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(body, &v); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &v, nil
}

// Tags returns the paginated list of profiles tagged in a video.
// Mirrors photo.Client.Tags.
func (c *Client) Tags(ctx context.Context, videoId string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + videoId + "/tags"
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

// Comments returns the paginated list of comments on a video.
// Mirrors photo.Client.Comments.
func (c *Client) Comments(ctx context.Context, videoId string, page int) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + videoId + "/comments"
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

	var comments comment.BulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
}

// AddComment posts a new comment on a video. Mirrors
// photo.Client.AddComment.
func (c *Client) AddComment(ctx context.Context, videoId, text, title string) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + videoId + "/comment"
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
