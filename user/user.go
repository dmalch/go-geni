// Package user wraps Geni's User resource and all /user/* endpoints —
// the authenticated user's account plus their followed listings,
// uploaded media, and metadata.
package user

import "encoding/json"

// User is Geni's User resource — the authenticated user's account.
// Distinct from profile.Profile (which describes a node in the tree);
// a single account may manage many profiles.
//
// The public docs list only `name` and `account_type` on this object,
// and the sandbox returns exactly those two — ID and Guid below are
// captured defensively in case production differs, but you should
// not depend on them being populated. The JSON decoder is permissive
// (extra fields are silently ignored).
type User struct {
	// ID is the user's identifier.
	ID string `json:"id,omitempty"`
	// Guid is the user's globally unique identifier.
	Guid string `json:"guid,omitempty"`
	// Name is the user's display name.
	Name string `json:"name,omitempty"`
	// AccountType is the subscription tier: "basic", "plus", or "pro".
	AccountType string `json:"account_type,omitempty"`
}

// LabelsResponse is the paginated envelope returned by
// [Client.Labels]. Each result is a label string — Geni's docs
// describe my-labels' results as "Array of Strings". Named with the
// Response suffix because the bare "Labels" identifier collides with
// Ginkgo's dot-imported `Labels` symbol in test files.
type LabelsResponse struct {
	Results  []string `json:"results,omitempty"`
	Page     int      `json:"page,omitempty"`
	NextPage string   `json:"next_page,omitempty"`
	PrevPage string   `json:"prev_page,omitempty"`
}

// Metadata is Geni's /user/metadata response. Data is the
// application-specific JSON blob the caller previously stored via
// [Client.UpdateMetadata]; the client leaves it as a raw message so
// callers can unmarshal into whatever structure their app uses.
type Metadata struct {
	Data json.RawMessage `json:"data,omitempty"`
}

// AddRequest is the body for [Client.Add]. Geni's /user/add endpoint
// requires all four fields, so none carry omitempty — an empty value
// is sent through for the server to reject explicitly.
type AddRequest struct {
	// Email is the new user's email address.
	Email string `json:"email"`
	// FirstName is the new user's first name.
	FirstName string `json:"first_name"`
	// LastName is the new user's last name.
	LastName string `json:"last_name"`
	// Gender is the new user's gender: "m", "f", or "u".
	Gender string `json:"gender"`
}

// AddResult is the outcome of [Client.Add]: the newly-created user
// plus the OAuth access token Geni issues for that account. The token
// arrives in the X-API-OAuth-access_token response header (not the
// body); it lets the caller immediately act on behalf of the new
// user.
type AddResult struct {
	// User is the created account, as returned in the response body.
	User *User
	// AccessToken is the new account's OAuth access token.
	AccessToken string
}
