package geni

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// FamilyNodes is the heterogeneous map of related entities returned by
// Geni's family-graph endpoints (immediate-family, ancestors). Keys are
// Geni-prefixed ids: "profile-..." or "union-...". Values are stored as
// raw JSON so callers decode lazily into [ProfileResponse] or
// [UnionResponse] via the accessor methods — this also leaves room for
// future node kinds (event-, document-, ...) without breaking the map.
type FamilyNodes map[string]json.RawMessage

// Profile decodes the node at id into a [ProfileResponse]. It errors if
// id does not name a profile node in the map.
func (n FamilyNodes) Profile(id string) (*ProfileResponse, error) {
	if !strings.HasPrefix(id, "profile-") {
		return nil, fmt.Errorf("not a profile id: %s", id)
	}
	raw, ok := n[id]
	if !ok {
		return nil, fmt.Errorf("profile node %s not found", id)
	}
	var p ProfileResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("decode profile node %s: %w", id, err)
	}
	return &p, nil
}

// Union decodes the node at id into a [UnionResponse]. It errors if id
// does not name a union node in the map.
func (n FamilyNodes) Union(id string) (*UnionResponse, error) {
	if !strings.HasPrefix(id, "union-") {
		return nil, fmt.Errorf("not a union id: %s", id)
	}
	raw, ok := n[id]
	if !ok {
		return nil, fmt.Errorf("union node %s not found", id)
	}
	var u UnionResponse
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, fmt.Errorf("decode union node %s: %w", id, err)
	}
	return &u, nil
}

// ProfileIds returns every map key with the "profile-" prefix.
func (n FamilyNodes) ProfileIds() []string {
	return n.idsWithPrefix("profile-")
}

// UnionIds returns every map key with the "union-" prefix.
func (n FamilyNodes) UnionIds() []string {
	return n.idsWithPrefix("union-")
}

func (n FamilyNodes) idsWithPrefix(prefix string) []string {
	out := make([]string, 0, len(n))
	for k := range n {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out
}

// FamilyResponse is the shape returned by Geni's family-graph endpoints
// (immediate-family, ancestors). Focus is the profile the call was
// anchored on, embedded inline by the server. Related profiles and
// unions live in Nodes.
type FamilyResponse struct {
	Focus *ProfileResponse `json:"focus,omitempty"`
	Nodes FamilyNodes      `json:"nodes,omitempty"`
}

// PathType is the value of the path_type query parameter on
// [Client.GetPathTo].
type PathType string

const (
	PathTypeClosest PathType = "closest"
	PathTypeBlood   PathType = "blood"
	PathTypeInlaw   PathType = "inlaw"
)

// PathStatus is the server-side computation status of a path-to call.
// Geni's path-to endpoint may return PathStatusPending for paths that
// have not been computed yet; the caller is expected to back off and
// re-issue the same request.
type PathStatus string

const (
	PathStatusPending    PathStatus = "pending"
	PathStatusDone       PathStatus = "done"
	PathStatusOverloaded PathStatus = "overloaded"
	PathStatusNotFound   PathStatus = "not found"
)

// PathRelation is one hop along a path-to result.
type PathRelation struct {
	Id       string `json:"id,omitempty"`
	Relation string `json:"relation,omitempty"`
	NextId   string `json:"next_id,omitempty"`
}

// PathToResponse is the shape returned by [Client.GetPathTo]. Status
// must be inspected before treating Relations as authoritative — a
// PathStatusPending response carries no relations.
type PathToResponse struct {
	Relations    []PathRelation `json:"relations,omitempty"`
	Relationship string         `json:"relationship,omitempty"`
	Status       PathStatus     `json:"status,omitempty"`
}

// TreeOption customises an outgoing request for the family-graph and
// path-to endpoints. Options only set the parameters they understand;
// passing an option to a method that doesn't honor it is harmless.
type TreeOption func(*http.Request)

// WithGenerations sets the generations query parameter on
// [Client.GetAncestors]. Values ≤0 are a no-op; values >20 are clamped
// to 20 (the Geni-documented maximum).
func WithGenerations(n int) TreeOption {
	return func(r *http.Request) {
		if n <= 0 {
			return
		}
		if n > 20 {
			n = 20
		}
		setQueryParam(r, "generations", strconv.Itoa(n))
	}
}

// WithPathType sets the path_type query parameter on [Client.GetPathTo].
// An empty value is a no-op (Geni defaults to "closest").
func WithPathType(t PathType) TreeOption {
	return func(r *http.Request) {
		if t == "" {
			return
		}
		setQueryParam(r, "path_type", string(t))
	}
}

// WithRefresh forces a recomputation of a path-to result. The flag is
// only emitted when v is true.
func WithRefresh(v bool) TreeOption {
	return boolOption("refresh", v, true)
}

// WithSearch toggles the path-to search behavior. Geni defaults to true,
// so the parameter is only emitted when v is false (i.e. to opt out).
func WithSearch(v bool) TreeOption {
	return boolOption("search", v, false)
}

// WithSkipEmail suppresses the email notification path-to would otherwise
// send. Only emitted when v is true.
func WithSkipEmail(v bool) TreeOption {
	return boolOption("skip_email", v, true)
}

// WithSkipNotify suppresses the on-site notification path-to would
// otherwise send. Only emitted when v is true.
func WithSkipNotify(v bool) TreeOption {
	return boolOption("skip_notify", v, true)
}

// boolOption emits "<name>=<v>" only when v equals emitWhen — every
// caller passes the polarity Geni's server-side default does not already
// cover, so we never send no-op parameters.
func boolOption(name string, v, emitWhen bool) TreeOption {
	return func(r *http.Request) {
		if v != emitWhen {
			return
		}
		setQueryParam(r, name, strconv.FormatBool(v))
	}
}

func setQueryParam(r *http.Request, key, value string) {
	q := r.URL.Query()
	q.Set(key, value)
	r.URL.RawQuery = q.Encode()
}

// GetImmediateFamily fetches the one-hop family graph around profileId
// (parents, partners, children, siblings) as a [FamilyResponse]. The
// response's Nodes map is heterogeneous — use [FamilyNodes.Profile] and
// [FamilyNodes.Union] to decode individual entries.
func (c *Client) GetImmediateFamily(ctx context.Context, profileId string) (*FamilyResponse, error) {
	return c.getFamily(ctx, "api/"+profileId+"/immediate-family")
}

// GetAncestors fetches the ancestor graph rooted at profileId.
// [WithGenerations] controls depth; the Geni-documented maximum is 20
// generations and values above that are clamped client-side.
//
// The Geni sandbox is known to return 403 (surfaced as
// [ErrAccessDenied]) when the focus profile isn't attached to the
// calling user's verified tree — even with the "family" OAuth scope
// granted. The public docs don't describe the access rule. If you see
// unexpected 403s, try the call against a profile from
// [Client.GetManagedProfiles] before assuming a client bug.
func (c *Client) GetAncestors(ctx context.Context, profileId string, opts ...TreeOption) (*FamilyResponse, error) {
	return c.getFamily(ctx, "api/"+profileId+"/ancestors", opts...)
}

func (c *Client) getFamily(ctx context.Context, path string, opts ...TreeOption) (*FamilyResponse, error) {
	url := BaseURL(c.useSandboxEnv) + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, o := range opts {
		o(req)
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var family FamilyResponse
	if err := json.Unmarshal(body, &family); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &family, nil
}

// GetPathTo fetches the kinship path between fromId and toId. The call
// is asynchronous on Geni's side: a [PathStatusPending] response means
// the server is still computing and the caller should back off and
// re-issue. Geni's path-to also has side effects (email + on-site
// notifications) unless suppressed via [WithSkipEmail] / [WithSkipNotify].
func (c *Client) GetPathTo(ctx context.Context, fromId, toId string, opts ...TreeOption) (*PathToResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + fromId + "/path-to/" + toId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, o := range opts {
		o(req)
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var path PathToResponse
	if err := json.Unmarshal(body, &path); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &path, nil
}
