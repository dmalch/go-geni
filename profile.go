package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

func (c *Client) CreateProfile(ctx context.Context, request *profile.Request) (*profile.Profile, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/profile/add"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

// escapeString and escapeStringToUTF are thin wrappers preserved for
// existing call sites. The actual escape lives in transport.
func escapeString(s string) string      { return transport.EscapeStringToUTF(s) }
func escapeStringToUTF(s string) string { return transport.EscapeStringToUTF(s) }

func (c *Client) GetProfile(ctx context.Context, profileId string) (*profile.Profile, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	coalescer := &transport.BulkCoalescer[profile.Profile, profile.BulkResponse]{
		CurrentID: profileId,
		IDPrefix:  "profile",
		DecodeBulk: func(body []byte) (profile.BulkResponse, error) {
			var env profile.BulkResponse
			if err := json.Unmarshal(body, &env); err != nil {
				return env, err
			}
			return env, nil
		},
		ListResults: func(env profile.BulkResponse) []profile.Profile { return env.Results },
		IDOfResult:  func(p profile.Profile) string { return p.Id },
	}

	body, err := c.doRequest(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

func (c *Client) addProfileFieldsQueryParams(req *http.Request) {
	query := req.URL.Query()
	query.Add("fields", "id,guid,first_name,last_name,middle_name,maiden_name,display_name,nicknames,names,gender,title,suffix,occupation,birth,baptism,death,burial,cause_of_death,current_residence,about_me,detail_strings,unions,project_ids,is_alive,public,deleted,merged_into,updated_at,created_at")
	req.URL.RawQuery = query.Encode()
}

func (c *Client) fixResponse(profile *profile.Profile) {
	//The only_ids flag does not work for the profile endpoint, so we need to remove
	//the url from the Unions field.
	for i, union := range profile.Unions {
		profile.Unions[i] = strings.Replace(union, apiUrl(c.useSandboxEnv), "", 1)
	}
}

func (c *Client) GetProfiles(ctx context.Context, profileIds []string) (*profile.BulkResponse, error) {
	// Single-id fallback — see GetUnions for the Geni-side quirk.
	if len(profileIds) == 1 {
		one, err := c.GetProfile(ctx, profileIds[0])
		if err != nil {
			return nil, err
		}
		return &profile.BulkResponse{Results: []profile.Profile{*one}}, nil
	}

	url := BaseURL(c.useSandboxEnv) + "api/profile"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	query := req.URL.Query()
	query.Add("ids", strings.Join(profileIds, ","))
	req.URL.RawQuery = query.Encode()

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

	for i := range profiles.Results {
		c.fixResponse(&profiles.Results[i])
	}

	return &profiles, nil
}

func (c *Client) GetManagedProfiles(ctx context.Context, page int) (*profile.BulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user/managed-profiles"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	query := req.URL.Query()
	query.Add("page", strconv.Itoa(page))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.BulkResponse
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	// Iterate over the profiles and fix the response
	for i := range profile.Results {
		c.fixResponse(&profile.Results[i])
	}

	return &profile, nil
}

// GetProfileDocuments returns the paginated list of documents
// attached to a profile. page is 1-indexed; values ≤0 omit the
// parameter (Geni defaults to page 1). Max 50 per page.
func (c *Client) GetProfileDocuments(ctx context.Context, profileId string, page int) (*DocumentBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/documents"
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

	var documents DocumentBulkResponse
	if err := json.Unmarshal(body, &documents); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &documents, nil
}

// GetProfilePhotos returns the paginated list of photos attached to
// a profile. page is 1-indexed; values ≤0 omit the parameter (Geni
// defaults to page 1). Max 50 per page.
func (c *Client) GetProfilePhotos(ctx context.Context, profileId string, page int) (*PhotoBulkResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/photos"
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

	var photos PhotoBulkResponse
	if err := json.Unmarshal(body, &photos); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &photos, nil
}

func (c *Client) UpdateProfile(ctx context.Context, profileId string, request *profile.Request) (*profile.Profile, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/update"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

// WipeEventDates issues a targeted PATCH against /api/<resourceId>/update
// that nulls only the `date` sub-object of each named event (e.g. `birth`,
// `baptism`, `death`, `burial` on a profile; `marriage` or `divorce` on a
// union). Geni's API deep-merges nested objects per-key, which means
// sending `"end_month": null` inside an otherwise-populated `date` is a
// no-op — the only way to clear individual date sub-fields is to first
// wipe the whole `date` and then re-PATCH the desired subset (#94).
//
// The request body is hand-crafted to touch only the named events' `date`
// keys; it deliberately omits `location`, `name`, and `description` to
// avoid accidentally clearing those alongside the date.
func (c *Client) WipeEventDates(ctx context.Context, resourceId string, eventKeys []string) error {
	if len(eventKeys) == 0 {
		return nil
	}

	// "date": {} (empty object) is the only sentinel both the profile and
	// union endpoints honor as a date wipe. "date": null wipes on profile but
	// is silently ignored on union.
	payload := make(map[string]any, len(eventKeys))
	for _, key := range eventKeys {
		payload[key] = map[string]any{"date": map[string]any{}}
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := BaseURL(c.useSandboxEnv) + "api/" + resourceId + "/update"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.doRequest(ctx, req); err != nil {
		return err
	}
	return nil
}

type ResultResponse struct {
	Result string `json:"result,omitempty"`
}

func (c *Client) DeleteProfile(ctx context.Context, profileId string) error {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/delete"
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
	err = json.Unmarshal(body, &result)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return err
	}

	return nil
}

func (c *Client) AddPartner(ctx context.Context, profileId string) (*profile.Profile, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/add-partner"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

func (c *Client) AddChild(ctx context.Context, profileId string, opts ...AddOption) (*profile.Profile, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/add-child"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)
	for _, opt := range opts {
		opt(req)
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

func (c *Client) AddSibling(ctx context.Context, profileId string, opts ...AddOption) (*profile.Profile, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/add-sibling"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	c.addProfileFieldsQueryParams(req)
	for _, opt := range opts {
		opt(req)
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile profile.Profile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	c.fixResponse(&profile)

	return &profile, nil
}

func (c *Client) MergeProfiles(ctx context.Context, profile1Id, profile2Id string) error {
	url := BaseURL(c.useSandboxEnv) + "api/" + profile1Id + "/merge/" + profile2Id
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
	err = json.Unmarshal(body, &result)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return err
	}

	return nil
}
