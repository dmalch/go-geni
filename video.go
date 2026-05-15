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
)

// VideoRequest is the JSON-encoded body for [Client.UpdateVideo].
// All fields are optional; omitted fields leave the existing value
// in place.
type VideoRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Date        string `json:"date,omitempty"`
}

// VideoResponse is Geni's Video resource — a single uploaded video
// (or a link to an externally-hosted video) with metadata and tagging.
type VideoResponse struct {
	// Id is the video's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the video's legacy global identifier.
	Guid string `json:"guid,omitempty"`
	// Title is the video's title.
	Title string `json:"title,omitempty"`
	// Description is the video's description.
	Description string `json:"description,omitempty"`
	// Date is the video's date, as returned by Geni (string format).
	Date string `json:"date,omitempty"`
	// Attribution is the video's attribution string.
	Attribution string `json:"attribution,omitempty"`
	// ContentType is the original MIME type of the upload.
	ContentType string `json:"content_type,omitempty"`
	// Location is the video's optional location.
	Location *LocationElement `json:"location,omitempty"`
	// Tags is the list of profiles tagged in the video (urls or ids
	// depending on the `only_ids` query parameter).
	Tags []string `json:"tags,omitempty"`
	// Sizes maps Geni-defined size names to fully-qualified URLs.
	Sizes map[string]string `json:"sizes,omitempty"`
	// Url is the API URL for the video.
	Url string `json:"url,omitempty"`
	// CreatedAt / UpdatedAt are the resource lifecycle timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// VideoBulkResponse is the envelope returned by [Client.GetVideos].
type VideoBulkResponse struct {
	Results []VideoResponse `json:"results,omitempty"`
}

// CreateVideoOption customises an outgoing CreateVideo request.
type CreateVideoOption func(*createVideoOptions)

type createVideoOptions struct {
	description string
	date        string
}

// WithVideoDescription sets the video's description on upload.
func WithVideoDescription(desc string) CreateVideoOption {
	return func(o *createVideoOptions) { o.description = desc }
}

// WithVideoDate sets the video's date. Geni accepts a free-form date
// string here (the public docs describe it as "Date in JSON form"
// without specifying); callers should consult Geni's docs for the
// exact format they expect.
func WithVideoDate(date string) CreateVideoOption {
	return func(o *createVideoOptions) { o.date = date }
}

// CreateVideo uploads a new video to Geni. The endpoint expects
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
func (c *Client) CreateVideo(ctx context.Context, title, fileName string, file io.Reader, opts ...CreateVideoOption) (*VideoResponse, error) {
	if title == "" {
		return nil, errInvalidArg("CreateVideo: title is required")
	}

	options := createVideoOptions{}
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

	url := BaseURL(c.useSandboxEnv) + "api/video/add"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	respBody, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var video VideoResponse
	if err := json.Unmarshal(respBody, &video); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &video, nil
}

// GetVideo fetches a single video by id.
func (c *Client) GetVideo(ctx context.Context, videoId string) (*VideoResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var video VideoResponse
	if err := json.Unmarshal(body, &video); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &video, nil
}

// GetVideos fetches multiple videos in a single bulk request.
func (c *Client) GetVideos(ctx context.Context, videoIds []string) (*VideoBulkResponse, error) {
	// Single-id fallback — see GetUnions for the Geni-side quirk.
	if len(videoIds) == 1 {
		one, err := c.GetVideo(ctx, videoIds[0])
		if err != nil {
			return nil, err
		}
		return &VideoBulkResponse{Results: []VideoResponse{*one}}, nil
	}

	url := BaseURL(c.useSandboxEnv) + "api/video"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(videoIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var videos VideoBulkResponse
	if err := json.Unmarshal(body, &videos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &videos, nil
}

// UpdateVideo mutates the video's title / description / date. Body is
// JSON-encoded and run through escapeStringToUTF for UTF-8 safety,
// matching the other update endpoints in the package.
func (c *Client) UpdateVideo(ctx context.Context, videoId string, request *VideoRequest) (*VideoResponse, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var video VideoResponse
	if err := json.Unmarshal(body, &video); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &video, nil
}

// DeleteVideo deletes a video by id.
func (c *Client) DeleteVideo(ctx context.Context, videoId string) error {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/delete"
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

// TagVideo associates a profile with a video.
func (c *Client) TagVideo(ctx context.Context, videoId, profileId string) (*VideoResponse, error) {
	return c.videoTagAction(ctx, videoId, profileId, "tag")
}

// UntagVideo removes a profile-tag from a video.
func (c *Client) UntagVideo(ctx context.Context, videoId, profileId string) (*VideoResponse, error) {
	return c.videoTagAction(ctx, videoId, profileId, "untag")
}

func (c *Client) videoTagAction(ctx context.Context, videoId, profileId, action string) (*VideoResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/" + action + "/" + profileId
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var video VideoResponse
	if err := json.Unmarshal(body, &video); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &video, nil
}

// GetVideoTags returns the paginated list of profiles tagged in a
// video. Mirrors [Client.GetPhotoTags].
func (c *Client) GetVideoTags(ctx context.Context, videoId string, page int) (*ProfileBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/tags"
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

// GetVideoComments returns the paginated list of comments on a video.
// Mirrors [Client.GetPhotoComments].
func (c *Client) GetVideoComments(ctx context.Context, videoId string, page int) (*CommentBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/comments"
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

	var comments CommentBulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
}

// AddVideoComment posts a new comment on a video. Mirrors
// [Client.AddPhotoComment].
func (c *Client) AddVideoComment(ctx context.Context, videoId, text, title string) (*CommentBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + videoId + "/comment"
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

	var comments CommentBulkResponse
	if err := json.Unmarshal(body, &comments); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &comments, nil
}
