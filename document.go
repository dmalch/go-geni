package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"log/slog"

	"github.com/dmalch/go-geni/comment"
	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/project"
	"github.com/dmalch/go-geni/transport"
)

func (c *Client) CreateDocument(ctx context.Context, request *document.Request) (*document.Document, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/document/add"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var document document.Document
	err = json.Unmarshal(body, &document)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &document, nil
}

func (c *Client) GetDocument(ctx context.Context, documentId string) (*document.Document, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[document.Document, document.BulkResponse]{
		CurrentID: documentId,
		IDPrefix:  "document",
		DecodeBulk: func(body []byte) (document.BulkResponse, error) {
			var env document.BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env document.BulkResponse) []document.Document { return env.Results },
		IDOfResult:  func(d document.Document) string { return d.Id },
	}

	body, err := c.doRequest(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var document document.Document
	err = json.Unmarshal(body, &document)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &document, nil
}

func (c *Client) GetDocuments(ctx context.Context, documentIds []string) (*document.BulkResponse, error) {
	// Single-id fallback — see GetUnions for the Geni-side quirk.
	if len(documentIds) == 1 {
		one, err := c.GetDocument(ctx, documentIds[0])
		if err != nil {
			return nil, err
		}
		return &document.BulkResponse{Results: []document.Document{*one}}, nil
	}

	url := BaseURL(c.useSandboxEnv) + "api/document"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(documentIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var document document.BulkResponse
	err = json.Unmarshal(body, &document)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &document, nil
}

func (c *Client) GetUploadedDocuments(ctx context.Context, page int) (*document.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/uploaded-documents"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("page", strconv.Itoa(page))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var document document.BulkResponse
	err = json.Unmarshal(body, &document)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &document, nil
}

func (c *Client) DeleteDocument(ctx context.Context, documentId string) error {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/delete"
	req, err := http.NewRequest(http.MethodPost, url, nil)

	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	var result transport.Result
	err = json.Unmarshal(body, &result)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return err
	}

	return nil
}

func (c *Client) UpdateDocument(ctx context.Context, documentId string, request *document.Request) (*document.Document, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/update"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var document document.Document
	err = json.Unmarshal(body, &document)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &document, nil
}

func (c *Client) TagDocument(ctx context.Context, documentId, profileId string) (*profile.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/tag/" + profileId

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profiles profile.BulkResponse
	err = json.Unmarshal(body, &profiles)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &profiles, nil
}

// GetDocumentComments returns the paginated list of comments on a
// document. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 comments per page.
func (c *Client) GetDocumentComments(ctx context.Context, documentId string, page int) (*comment.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/comments"
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

// AddDocumentComment posts a new comment on a document. text is the
// comment body and is required by Geni; title is optional and may be
// the empty string. The response is a [CommentBulkResponse] — the
// updated paginated comment list.
func (c *Client) AddDocumentComment(ctx context.Context, documentId, text, title string) (*comment.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/comment"
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

// GetDocumentProjects returns the paginated list of projects a
// document belongs to. page is 1-indexed; values ≤0 omit the parameter
// (Geni defaults to page 1). Max 50 projects per page.
func (c *Client) GetDocumentProjects(ctx context.Context, documentId string, page int) (*project.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/projects"
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

	var projects project.BulkResponse
	if err := json.Unmarshal(body, &projects); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &projects, nil
}

// GetDocumentTags returns the paginated list of profiles tagged in a
// document. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 tags per page. Symmetric with
// [Client.GetPhotoTags].
func (c *Client) GetDocumentTags(ctx context.Context, documentId string, page int) (*profile.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/tags"
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

func (c *Client) UntagDocument(ctx context.Context, documentId, profileId string) (*profile.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + documentId + "/untag/" + profileId

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profiles profile.BulkResponse
	err = json.Unmarshal(body, &profiles)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &profiles, nil
}
