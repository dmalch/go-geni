// Package project carries the Project resource's wire types. Project
// API methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into this package in a
// later PR; this PR lifts only the types.
package project

// BulkResponse is the paginated envelope returned by project-bulk
// endpoints (followed-projects, document/{id}/projects, …).
type BulkResponse struct {
	Results  []Project `json:"results,omitempty"`
	Page     int       `json:"page,omitempty"`
	NextPage string    `json:"next_page,omitempty"`
	PrevPage string    `json:"prev_page,omitempty"`
}

// Project is Geni's Project resource — a collaborative grouping
// (e.g. "Ancestors of …", "World War II Veterans").
type Project struct {
	// Id is the project's id.
	Id string `json:"id,omitempty"`
	// Name is the project's name.
	Name string `json:"name,omitempty"`
	// Description is the project's description.
	Description *string `json:"description,omitempty"`
	// UpdatedAt is the timestamp of when the project was last updated.
	UpdatedAt string `json:"updated_at,omitempty"`
	// CreatedAt is the timestamp of when the project was created.
	CreatedAt string `json:"created_at,omitempty"`
}
