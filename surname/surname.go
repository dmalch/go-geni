// Package surname wraps Geni's Surname resource — a tag for a family
// name. Surnames are auto-created by Geni from profile lastnames; this
// package exposes a single Get plus the two surname-scoped profile
// listings (Followers, Profiles).
package surname

// Surname is Geni's Surname resource. Used as the parent for
// surname-scoped collections (followers, profiles).
type Surname struct {
	// Id is the surname's identifier.
	Id string `json:"id,omitempty"`
	// Description is the surname's free-text description.
	Description string `json:"description,omitempty"`
	// SluggedName is the surname rendered as a URL-safe slug (e.g.
	// "smith" for "Smith").
	SluggedName string `json:"slugged_name,omitempty"`
	// Url is the API URL for the surname.
	Url string `json:"url,omitempty"`
}

// BulkResponse is the paginated envelope returned by surname-bulk
// endpoints (currently the /user/followed-surnames listing, exposed
// via the root user/ surface). A natural home for any future
// surname-bulk endpoints too.
type BulkResponse struct {
	Results  []Surname `json:"results,omitempty"`
	Page     int       `json:"page,omitempty"`
	NextPage string    `json:"next_page,omitempty"`
	PrevPage string    `json:"prev_page,omitempty"`
}
