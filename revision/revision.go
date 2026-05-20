// Package revision wraps Geni's Revision resource — a single edit in
// a profile or tree's history. Revisions are read-only.
package revision

// Revision is Geni's Revision resource.
type Revision struct {
	// ID is the revision's identifier.
	ID string `json:"id,omitempty"`
	// Guid is the revision's globally unique identifier.
	Guid string `json:"guid,omitempty"`
	// Action describes what the revision did.
	Action string `json:"action,omitempty"`
	// DateLocal is the date of the revision in the local timezone.
	DateLocal string `json:"date_local,omitempty"`
	// TimeLocal is the time of the revision in the local timezone.
	TimeLocal string `json:"time_local,omitempty"`
	// Timestamp is the server-time timestamp.
	Timestamp string `json:"timestamp,omitempty"`
	// Story is an HTML rendering of the full revision description.
	Story string `json:"story,omitempty"`
}

// BulkResponse is the envelope returned by [Client.GetBulk].
type BulkResponse struct {
	Results []Revision `json:"results,omitempty"`
}
