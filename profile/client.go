package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the profile endpoints.
//
// The profile package is foundational: every other resource
// sub-package imports it (for Profile, the shared value objects, and
// AddOption), so profile/ itself imports nothing from this module but
// transport/. Endpoints that return a non-profile resource — adding a
// photo to a profile, listing a profile's documents, comparing two
// profiles' family graphs — therefore live on the resource they
// return (photo/, document/, tree/), not here.
type Client struct {
	transport *transport.Client
}

// NewClient returns a profile Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

func (c *Client) marshalEscaped(request any) (string, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return "", err
	}
	return transport.EscapeStringToUTF(strings.ReplaceAll(string(jsonBody), "\\\\", "\\")), nil
}

func (c *Client) decode(body []byte) (*Profile, error) {
	var p Profile
	if err := json.Unmarshal(body, &p); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	StripURLs(&p, c.transport.APIURL())
	return &p, nil
}

// Create creates a new profile from the supplied request.
func (c *Client) Create(ctx context.Context, request *Request) (*Profile, error) {
	jsonStr, err := c.marshalEscaped(request)
	if err != nil {
		return nil, err
	}

	url := c.transport.BaseURL() + "api/profile/add"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// Get fetches a single profile by id. Concurrent Get calls are
// coalesced into one bulk request via transport.BulkCoalescer.
func (c *Client) Get(ctx context.Context, profileId string) (*Profile, error) {
	url := c.transport.BaseURL() + "api/" + profileId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	coalescer := &transport.BulkCoalescer[Profile, BulkResponse]{
		CurrentID: profileId,
		IDPrefix:  "profile",
		DecodeBulk: func(body []byte) (BulkResponse, error) {
			var env BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env BulkResponse) []Profile { return env.Results },
		IDOfResult:  func(p Profile) string { return p.ID },
	}

	body, err := c.transport.Do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// GetBulk fetches multiple profiles in one request. The single-id
// fallback (Geni's bulk dispatcher returns empty for len(ids)==1) is
// preserved verbatim.
func (c *Client) GetBulk(ctx context.Context, profileIds []string) (*BulkResponse, error) {
	if len(profileIds) == 1 {
		one, err := c.Get(ctx, profileIds[0])
		if err != nil {
			return nil, err
		}
		return &BulkResponse{Results: []Profile{*one}}, nil
	}

	url := c.transport.BaseURL() + "api/profile"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	query := req.URL.Query()
	query.Add("ids", strings.Join(profileIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decodeBulk(body)
}

func (c *Client) decodeBulk(body []byte) (*BulkResponse, error) {
	var profiles BulkResponse
	if err := json.Unmarshal(body, &profiles); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	apiURL := c.transport.APIURL()
	for i := range profiles.Results {
		StripURLs(&profiles.Results[i], apiURL)
	}
	return &profiles, nil
}

// Update mutates a profile's full field set. Body is JSON-encoded and
// run through transport.EscapeStringToUTF for UTF-8 safety.
func (c *Client) Update(ctx context.Context, profileId string, request *Request) (*Profile, error) {
	jsonStr, err := c.marshalEscaped(request)
	if err != nil {
		return nil, err
	}

	url := c.transport.BaseURL() + "api/" + profileId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// UpdateBasics updates the "basics and about" subset of profile
// fields — a narrower target than Update. The request body uses the
// same Request shape but only fields in the basics/about scope take
// effect.
func (c *Client) UpdateBasics(ctx context.Context, profileId string, request *Request) (*Profile, error) {
	jsonStr, err := c.marshalEscaped(request)
	if err != nil {
		return nil, err
	}

	url := c.transport.BaseURL() + "api/" + profileId + "/update-basics"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// Delete deletes a profile by id.
func (c *Client) Delete(ctx context.Context, profileId string) error {
	url := c.transport.BaseURL() + "api/" + profileId + "/delete"
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

// AddPartner creates a new partner profile attached to profileId and
// returns it.
func (c *Client) AddPartner(ctx context.Context, profileId string) (*Profile, error) {
	return c.addRelative(ctx, profileId, "add-partner")
}

// AddChild creates a new child profile attached to profileId and
// returns it. [WithModifier] selects "adopt" or "foster".
func (c *Client) AddChild(ctx context.Context, profileId string, opts ...AddOption) (*Profile, error) {
	return c.addRelative(ctx, profileId, "add-child", opts...)
}

// AddSibling creates a new sibling profile attached to profileId and
// returns it. [WithModifier] selects "adopt" or "foster".
func (c *Client) AddSibling(ctx context.Context, profileId string, opts ...AddOption) (*Profile, error) {
	return c.addRelative(ctx, profileId, "add-sibling", opts...)
}

func (c *Client) addRelative(ctx context.Context, profileId, action string, opts ...AddOption) (*Profile, error) {
	url := c.transport.BaseURL() + "api/" + profileId + "/" + action
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)
	for _, opt := range opts {
		opt(req)
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// AddParent creates and attaches a new parent profile to profileId.
// The request body is the same shape as Create — names, gender,
// birth, death, etc. [WithModifier] records an adopted/foster
// relationship.
func (c *Client) AddParent(ctx context.Context, profileId string, request *Request, opts ...AddOption) (*Profile, error) {
	jsonStr, err := c.marshalEscaped(request)
	if err != nil {
		return nil, err
	}

	url := c.transport.BaseURL() + "api/" + profileId + "/add-parent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)
	for _, opt := range opts {
		opt(req)
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// Follow makes the calling user follow the named profile. Returns the
// followed profile.
func (c *Client) Follow(ctx context.Context, profileId string) (*Profile, error) {
	return c.followAction(ctx, profileId, "follow")
}

// Unfollow reverses Follow. Returns the unfollowed profile.
func (c *Client) Unfollow(ctx context.Context, profileId string) (*Profile, error) {
	return c.followAction(ctx, profileId, "unfollow")
}

func (c *Client) followAction(ctx context.Context, profileId, action string) (*Profile, error) {
	url := c.transport.BaseURL() + "api/" + profileId + "/" + action
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}
	AddFields(req)

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return c.decode(body)
}

// Merge merges profile2Id into profile1Id. When the caller has edit
// permission on both, the merge happens immediately; otherwise Geni
// records a request-merge (surfaced on Profile.MergePending /
// MergeNote).
func (c *Client) Merge(ctx context.Context, profile1Id, profile2Id string) error {
	url := c.transport.BaseURL() + "api/" + profile1Id + "/merge/" + profile2Id
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

// WipeEventDates issues a targeted PATCH against /api/<resourceId>/update
// that nulls only the `date` sub-object of each named event (e.g. `birth`,
// `baptism`, `death`, `burial` on a profile; `marriage` or `divorce` on a
// union — resourceId may be either a profile or a union id). Geni's API
// deep-merges nested objects per-key, which means sending
// `"end_month": null` inside an otherwise-populated `date` is a no-op —
// the only way to clear individual date sub-fields is to first wipe the
// whole `date` and then re-PATCH the desired subset (#94).
//
// The request body is hand-crafted to touch only the named events' `date`
// keys; it deliberately omits `location`, `name`, and `description` to
// avoid accidentally clearing those alongside the date.
func (c *Client) WipeEventDates(ctx context.Context, resourceId string, eventKeys []string) error {
	if len(eventKeys) == 0 {
		return nil
	}

	// "date": {} (empty object) is the only sentinel both the profile and
	// union endpoints honor as a date wipe. "date": null wipes on profile
	// but is silently ignored on union.
	payload := make(map[string]any, len(eventKeys))
	for _, key := range eventKeys {
		payload[key] = map[string]any{"date": map[string]any{}}
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := c.transport.BaseURL() + "api/" + resourceId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.transport.Do(ctx, req, nil); err != nil {
		return err
	}
	return nil
}
