package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dmalch/go-geni/profile"
)

// ProfileComparison is the response shape of [Client.CompareProfiles].
// Geni returns immediate-family graphs for both profiles in one call;
// each Results entry mirrors what GetImmediateFamily would return for
// one of the two profiles, in the order requested.
type ProfileComparison struct {
	Results []FamilyResponse `json:"results,omitempty"`
}

// MugshotRequest is the JSON-encoded body for [Client.AddProfileMugshot].
// Either File or PhotoId is required — File uploads a new image via
// Base64 (the JSON path; not the multipart one), PhotoId reuses an
// existing photo as the mugshot.
type MugshotRequest struct {
	// File is the Base64-encoded image to upload as the mugshot.
	// Mutually exclusive with PhotoId; required when PhotoId is not
	// set.
	File *string `json:"file,omitempty"`
	// PhotoId reuses an existing photo as the mugshot. Mutually
	// exclusive with File; required when File is not set.
	PhotoId *string `json:"photo_id,omitempty"`
	// Title, Description, Date, AlbumId are all optional.
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Date        *string `json:"date,omitempty"`
	AlbumId     *string `json:"album_id,omitempty"`
}

// FollowProfile makes the calling user follow the named profile.
// Returns the followed profile.
func (c *Client) FollowProfile(ctx context.Context, profileId string) (*profile.Profile, error) {
	return c.profileFollowAction(ctx, profileId, "follow")
}

// UnfollowProfile reverses FollowProfile. Returns the unfollowed
// profile.
func (c *Client) UnfollowProfile(ctx context.Context, profileId string) (*profile.Profile, error) {
	return c.profileFollowAction(ctx, profileId, "unfollow")
}

func (c *Client) profileFollowAction(ctx context.Context, profileId, action string) (*profile.Profile, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/" + action
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
	if err := json.Unmarshal(body, &profile); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	c.fixResponse(&profile)
	return &profile, nil
}

// CompareProfiles fetches the immediate-family graphs of both
// profiles in a single call. The returned Results slice has two
// entries, one per profile, in the order they were requested.
func (c *Client) CompareProfiles(ctx context.Context, profile1Id, profile2Id string) (*ProfileComparison, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + profile1Id + "/compare/" + profile2Id
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var cmp ProfileComparison
	if err := json.Unmarshal(body, &cmp); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &cmp, nil
}

// AddParent creates and attaches a new parent profile to the named
// profile. The request body is the same shape as CreateProfile —
// names, gender, birth, death, etc. WithModifier("adopt" | "foster")
// records an adopted or foster relationship.
//
// Counterpart to AddChild / AddPartner / AddSibling.
func (c *Client) AddParent(ctx context.Context, profileId string, request *profile.Request, opts ...AddOption) (*profile.Profile, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/add-parent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
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
	if err := json.Unmarshal(body, &profile); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	c.fixResponse(&profile)
	return &profile, nil
}

// UpdateProfileBasics updates the "basics and about" subset of
// profile fields — a narrower target than UpdateProfile. The
// request body uses the same ProfileRequest shape but only fields
// in the basics/about scope take effect.
func (c *Client) UpdateProfileBasics(ctx context.Context, profileId string, request *profile.Request) (*profile.Profile, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/update-basics"
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
	if err := json.Unmarshal(body, &profile); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	c.fixResponse(&profile)
	return &profile, nil
}

// AddProfilePhoto attaches a new photo to a profile. Returns the
// created Photo. Unlike [Client.CreatePhoto] (which uses
// multipart/form-data), this endpoint takes a JSON body with the
// file encoded as Base64 in PhotoRequest.File.
func (c *Client) AddProfilePhoto(ctx context.Context, profileId string, request *PhotoRequest) (*PhotoResponse, error) {
	return doJSONPost[PhotoResponse](ctx, c, profileId, "add-photo", request)
}

// AddProfileVideo attaches a new video to a profile. Returns the
// created Video. Unlike [Client.CreateVideo] (which uses
// multipart/form-data), this endpoint takes a JSON body with the
// file encoded as Base64 in VideoRequest.File.
func (c *Client) AddProfileVideo(ctx context.Context, profileId string, request *VideoRequest) (*VideoResponse, error) {
	return doJSONPost[VideoResponse](ctx, c, profileId, "add-video", request)
}

// AddProfileDocument attaches a new document to a profile. Returns
// the created Document. Accepts the same DocumentRequest used by
// [Client.CreateDocument] — text/file/source_url are mutually
// exclusive content sources.
func (c *Client) AddProfileDocument(ctx context.Context, profileId string, request *DocumentRequest) (*DocumentResponse, error) {
	return doJSONPost[DocumentResponse](ctx, c, profileId, "add-document", request)
}

// AddProfileMugshot sets a profile's mugshot — either by uploading a
// new image (MugshotRequest.File, Base64) or by reusing an existing
// photo (MugshotRequest.PhotoId).
func (c *Client) AddProfileMugshot(ctx context.Context, profileId string, request *MugshotRequest) (*PhotoResponse, error) {
	return doJSONPost[PhotoResponse](ctx, c, profileId, "add-mugshot", request)
}

// doJSONPost is the shared body of AddProfilePhoto / AddProfileVideo
// / AddProfileDocument / AddProfileMugshot. All four POST a
// JSON-encoded body to /api/<profileId>/<action> and decode the
// response into the resource's typed envelope. The escapeStringToUTF
// pass is preserved (Geni's API historically mishandled raw UTF-8 in
// JSON request bodies).
func doJSONPost[T any](ctx context.Context, c *Client, profileId, action string, request any) (*T, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}
	jsonStr := escapeString(strings.ReplaceAll(string(jsonBody), "\\\\", "\\"))

	url := BaseURL(c.useSandboxEnv) + "api/" + profileId + "/" + action
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &out, nil
}
