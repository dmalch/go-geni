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
