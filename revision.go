package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// Revision is Geni's Revision resource — a single edit in a profile
// or tree's history.
type Revision struct {
	// Id is the revision's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the revision's globally unique identifier.
	Guid string `json:"guid,omitempty"`
	// Action describes what the revision did.
	Action string `json:"action,omitempty"`
	// DateLocal is the date of the revision in the local timezone.
	DateLocal string `json:"date_local,omitempty"`
	// TimeLocal is the time of the revision in the local timezone.
	TimeLocal string `json:"time_local,omitempty"`
	// Timestamp is the server-time timestamp.
	Timestamp string `json:"timestamp,omitempty"`
	// Story is an HTML rendering of the full revision description.
	Story string `json:"story,omitempty"`
}

// RevisionBulkResponse is the envelope returned by
// [Client.GetRevisions].
type RevisionBulkResponse struct {
	Results []Revision `json:"results,omitempty"`
}

// GetRevision fetches a single revision by id.
func (c *Client) GetRevision(ctx context.Context, revisionId string) (*Revision, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + revisionId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
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

// GetRevisions fetches multiple revisions in one call. Mirrors the
// single-id bulk fallback used by the other bulk Get* methods —
// when len(ids) == 1 the request goes through the singular GetRevision
// path so Geni's bulk dispatcher quirk doesn't return empty results.
func (c *Client) GetRevisions(ctx context.Context, revisionIds []string) (*RevisionBulkResponse, error) {
	if len(revisionIds) == 1 {
		one, err := c.GetRevision(ctx, revisionIds[0])
		if err != nil {
			return nil, err
		}
		return &RevisionBulkResponse{Results: []Revision{*one}}, nil
	}

	url := BaseURL(c.useSandboxEnv) + "api/revision"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(revisionIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var revisions RevisionBulkResponse
	if err := json.Unmarshal(body, &revisions); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &revisions, nil
}
