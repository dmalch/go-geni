// Package tree wraps Geni's family-graph endpoints — immediate-family,
// ancestors, and path-to. These walk the relationship graph rather
// than fetching a single resource, so they live in their own package
// rather than under profile/.
package tree

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/union"
)

// FamilyNodes is the heterogeneous map of related entities returned by
// Geni's family-graph endpoints (immediate-family, ancestors). Keys
// are Geni-prefixed ids: "profile-..." or "union-...". Values are
// stored as raw JSON so callers decode lazily into [profile.Profile]
// or [union.Union] via the accessor methods — this also leaves room
// for future node kinds (event-, document-, ...) without breaking the
// map.
type FamilyNodes map[string]json.RawMessage

// Profile decodes the node at id into a [profile.Profile]. It errors
// if id does not name a profile node in the map.
func (n FamilyNodes) Profile(id string) (*profile.Profile, error) {
	if !strings.HasPrefix(id, "profile-") {
		return nil, fmt.Errorf("not a profile id: %s", id)
	}
	raw, ok := n[id]
	if !ok {
		return nil, fmt.Errorf("profile node %s not found", id)
	}
	var p profile.Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("decode profile node %s: %w", id, err)
	}
	return &p, nil
}

// Union decodes the node at id into a [union.Union]. It errors if id
// does not name a union node in the map.
func (n FamilyNodes) Union(id string) (*union.Union, error) {
	if !strings.HasPrefix(id, "union-") {
		return nil, fmt.Errorf("not a union id: %s", id)
	}
	raw, ok := n[id]
	if !ok {
		return nil, fmt.Errorf("union node %s not found", id)
	}
	var u union.Union
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

// FamilyResponse is the shape returned by Geni's family-graph
// endpoints (immediate-family, ancestors). Focus is the profile the
// call was anchored on, embedded inline by the server. Related
// profiles and unions live in Nodes.
type FamilyResponse struct {
	Focus *profile.Profile `json:"focus,omitempty"`
	Nodes FamilyNodes      `json:"nodes,omitempty"`
}

// PathType is the value of the path_type query parameter on
// [Client.PathTo].
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
	ID       string `json:"id,omitempty"`
	Relation string `json:"relation,omitempty"`
	NextId   string `json:"next_id,omitempty"`
}

// PathToResponse is the shape returned by [Client.PathTo]. Status
// must be inspected before treating Relations as authoritative — a
// PathStatusPending response carries no relations.
type PathToResponse struct {
	Relations    []PathRelation `json:"relations,omitempty"`
	Relationship string         `json:"relationship,omitempty"`
	Status       PathStatus     `json:"status,omitempty"`
}

// Comparison is the response shape of [Client.Compare]. Geni returns
// immediate-family graphs for both profiles in one call; each Results
// entry mirrors what [Client.ImmediateFamily] would return for one of
// the two profiles, in the order requested.
type Comparison struct {
	Results []FamilyResponse `json:"results,omitempty"`
}
