package document

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/comment"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/project"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the document endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a document Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Create posts a new document. The Request fields describe the upload —
// title, description, content type, optional Base64-encoded file, etc.
func (c *Client) Create(ctx context.Context, request *Request) (*Document, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/document/add"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &d, nil
}

// Get fetches a single document by id. Concurrent Get calls are
// coalesced into one bulk request via transport.BulkCoalescer.
func (c *Client) Get(ctx context.Context, documentId string) (*Document, error) {
	url := c.transport.BaseURL() + "api/" + documentId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[Document, BulkResponse]{
		CurrentID: documentId,
		IDPrefix:  "document",
		DecodeBulk: func(body []byte) (BulkResponse, error) {
			var env BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env BulkResponse) []Document { return env.Results },
		IDOfResult:  func(d Document) string { return d.ID },
	}

	body, err := c.transport.Do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &d, nil
}

// GetBulk fetches multiple documents in a single bulk request. The
// single-id fallback (Geni's bulk dispatcher returns empty for
// len(ids)==1) is preserved verbatim.
func (c *Client) GetBulk(ctx context.Context, documentIds []string) (*BulkResponse, error) {
	if len(documentIds) == 1 {
		one, err := c.Get(ctx, documentIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Document{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/document"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(documentIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var docs BulkResponse
	if err := json.Unmarshal(body, &docs); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &docs, nil
}

// Delete deletes a document by id.
func (c *Client) Delete(ctx context.Context, documentId string) error {
	url := c.transport.BaseURL() + "api/" + documentId + "/delete"
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

// Update mutates the document's metadata. Body is JSON-encoded and
// run through transport.EscapeStringToUTF for UTF-8 safety.
func (c *Client) Update(ctx context.Context, documentId string, request *Request) (*Document, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + documentId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &d, nil
}

// Tag associates a profile with a document. Returns the updated
// paginated profile-tags list.
func (c *Client) Tag(ctx context.Context, documentId, profileId string) (*profile.BulkResponse, error) {
	return c.tagAction(ctx, documentId, profileId, "tag")
}

// Untag removes a profile-tag from a document. Returns the updated
// paginated profile-tags list.
func (c *Client) Untag(ctx context.Context, documentId, profileId string) (*profile.BulkResponse, error) {
	return c.tagAction(ctx, documentId, profileId, "untag")
}

func (c *Client) tagAction(ctx context.Context, documentId, profileId, action string) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + documentId + "/" + action + "/" + profileId
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
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
	return &profiles, nil
}

// Tags returns the paginated list of profiles tagged in a document.
// page is 1-indexed; values ≤0 omit the parameter (Geni defaults to
// page 1). Max 50 tags per page. Symmetric with photo.Client.Tags.
func (c *Client) Tags(ctx context.Context, documentId string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + documentId + "/tags"
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

// Comments returns the paginated list of comments on a document.
// page is 1-indexed; values ≤0 omit the parameter (Geni defaults to
// page 1). Max 50 comments per page.
func (c *Client) Comments(ctx context.Context, documentId string, page int) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + documentId + "/comments"
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

// AddComment posts a new comment on a document. text is the comment
// body and is required by Geni; title is optional and may be the
// empty string. The response is a [comment.BulkResponse] — the
// updated paginated comment list.
func (c *Client) AddComment(ctx context.Context, documentId, text, title string) (*comment.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + documentId + "/comment"
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

// ForProfile returns the paginated list of documents attached to a
// profile. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 per page.
func (c *Client) ForProfile(ctx context.Context, profileId string, page int) (*BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + profileId + "/documents"
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

	var docs BulkResponse
	if err := json.Unmarshal(body, &docs); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &docs, nil
}

// AddToProfile attaches a new document to a profile. Accepts the same
// Request used by Create — text/file/source_url are mutually
// exclusive content sources.
func (c *Client) AddToProfile(ctx context.Context, profileId string, request *Request) (*Document, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + profileId + "/add-document"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &d, nil
}

// AddToProject tags a document into a project. Returns the bulk
// envelope of documents Geni associates with the project after the
// add. The endpoint is project-scoped (/api/<projectId>/add_documents)
// but the operation lives here so document and project don't import
// each other.
func (c *Client) AddToProject(ctx context.Context, documentId, projectId string) (*BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + projectId + "/add_documents"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", documentId)
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var docs BulkResponse
	if err := json.Unmarshal(body, &docs); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &docs, nil
}

// Projects returns the paginated list of projects a document belongs
// to. page is 1-indexed; values ≤0 omit the parameter (Geni defaults
// to page 1). Max 50 projects per page.
func (c *Client) Projects(ctx context.Context, documentId string, page int) (*project.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/" + documentId + "/projects"
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

	var projects project.BulkResponse
	if err := json.Unmarshal(body, &projects); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &projects, nil
}
