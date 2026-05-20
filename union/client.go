package union

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the union endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a union Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get fetches a single union by id. Concurrent Get calls are coalesced
// into one bulk request via transport.BulkCoalescer.
func (c *Client) Get(ctx context.Context, unionId string) (*Union, error) {
	url := c.transport.BaseURL() + "api/" + unionId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	coalescer := &transport.BulkCoalescer[Union, BulkResponse]{
		CurrentID: unionId,
		IDPrefix:  "union",
		DecodeBulk: func(body []byte) (BulkResponse, error) {
			var env BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env BulkResponse) []Union { return env.Results },
		IDOfResult:  func(u Union) string { return u.Id },
	}

	body, err := c.transport.Do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var u Union
	if err := json.Unmarshal(body, &u); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &u, nil
}

// GetBulk fetches multiple unions in a single bulk request.
//
// Geni's bulk endpoint returns an empty results array when `ids=`
// carries exactly one identifier — the server appears to route
// single-id bulk requests through a search/filter path rather than a
// fetch-by-id path. GetBulk falls back to the singular endpoint and
// wraps the result so the caller sees a consistent envelope
// regardless of input size.
func (c *Client) GetBulk(ctx context.Context, unionIds []string) (*BulkResponse, error) {
	if len(unionIds) == 1 {
		one, err := c.Get(ctx, unionIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Union{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/union"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(unionIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var unions BulkResponse
	if err := json.Unmarshal(body, &unions); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &unions, nil
}

// Update mutates a union's marriage / divorce events. Body is
// JSON-encoded and run through transport.EscapeStringToUTF for UTF-8
// safety.
func (c *Client) Update(ctx context.Context, unionId string, request *Request) (*Union, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := c.transport.BaseURL() + "api/" + unionId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var u Union
	if err := json.Unmarshal(body, &u); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &u, nil
}

// AddPartner adds a new partner profile to an existing union and
// returns the newly-created profile. Geni's public docs describe the
// response as a union, but the live API returns the new partner
// profile; refetch the union via [Client.Get] if you need the updated
// partner list.
func (c *Client) AddPartner(ctx context.Context, unionId string) (*profile.Profile, error) {
	url := c.transport.BaseURL() + "api/" + unionId + "/add-partner"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p profile.Profile
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	profile.StripURLs(&p, c.transport.APIURL())
	return &p, nil
}

// AddChild adds a new child profile to an existing union and returns
// the newly-created profile. [profile.WithModifier] selects "adopt"
// or "foster" to record an adopted/foster relationship — the modifier
// is stored on the union (in `adopted_children` / `foster_children`),
// so refetch via [Client.Get] to confirm it took effect.
func (c *Client) AddChild(ctx context.Context, unionId string, opts ...profile.AddOption) (*profile.Profile, error) {
	url := c.transport.BaseURL() + "api/" + unionId + "/add-child"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, opt := range opts {
		opt(req)
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var p profile.Profile
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	profile.StripURLs(&p, c.transport.APIURL())
	return &p, nil
}
