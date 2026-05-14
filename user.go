package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// User is Geni's User resource — the authenticated user's account.
// Distinct from ProfileResponse (which describes a node in the tree);
// a single account may manage many profiles.
//
// The public docs list only `name` and `account_type` on this object,
// and the sandbox returns exactly those two — Id and Guid below are
// captured defensively in case production differs, but you should
// not depend on them being populated. The JSON decoder is permissive
// (extra fields are silently ignored).
type User struct {
	// Id is the user's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the user's globally unique identifier.
	Guid string `json:"guid,omitempty"`
	// Name is the user's display name.
	Name string `json:"name,omitempty"`
	// AccountType is the subscription tier: "basic", "plus", or "pro".
	AccountType string `json:"account_type,omitempty"`
}

// GetUser returns the authenticated user — the account behind the
// OAuth token. Use this to ground "me" in API workflows that need to
// know who's calling (e.g. to filter Client.GetManagedProfiles results
// to ones the calling user actually owns).
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	url := BaseURL(c.useSandboxEnv) + "api/user"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &user, nil
}
