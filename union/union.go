// Package union carries the Union resource's wire types. Union API
// methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into this package in a
// later PR; this PR lifts only the types.
package union

import "github.com/dmalch/go-geni/profile"

// Request is the JSON-encoded body for UpdateUnion.
type Request struct {
	// Marriage date and location
	Marriage *profile.EventElement `json:"marriage,omitempty"`
	// Divorce date and location
	Divorce *profile.EventElement `json:"divorce,omitempty"`
}

// BulkResponse is the envelope returned by GetUnions.
type BulkResponse struct {
	Results []Union `json:"results,omitempty"`
}

// Union is Geni's Union resource — a relationship grouping that ties
// partners and children together.
type Union struct {
	// Id is the union's id.
	Id string `json:"id,omitempty"`
	// AdoptedChildren is a subset of the children array, indicating which children are adopted
	AdoptedChildren []string `json:"adopted_children,omitempty"`
	// Children is an array of children in the union (urls or ids, if requested)
	Children []string `json:"children,omitempty"`
	// FosterChildren is a subset of the children array, indicating which children are foster
	FosterChildren []string `json:"foster_children,omitempty"`
	// Partners is an array of partners in the union (urls or ids, if requested)
	Partners []string `json:"partners,omitempty"`
	// Marriage date and location
	Marriage *profile.EventElement `json:"marriage,omitempty"`
	// Divorce date and location
	Divorce *profile.EventElement `json:"divorce,omitempty"`
	// Status of the union (spouse|ex_spouse)
	Status string `json:"status,omitempty"`
}
