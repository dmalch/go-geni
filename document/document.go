// Package document carries the Document resource's wire types.
// Document API methods still live on github.com/dmalch/go-geni's
// root Client during the pre-1.0 reshape and migrate into this
// package in a later PR; this PR lifts only the types.
package document

import "github.com/dmalch/go-geni/profile"

// Request is the JSON-encoded body for CreateDocument / UpdateDocument.
type Request struct {
	// Title is the document's title
	Title string `json:"title,omitempty"`
	// Description is the document's description
	Description *string `json:"description,omitempty"`
	// ContentType is the document's content type
	ContentType *string `json:"content_type,omitempty"`
	// Date is the document's date
	Date *profile.DateElement `json:"date,omitempty"`
	// Location is the document's location
	Location *profile.LocationElement `json:"location,omitempty"`
	// Labels is the document's comma separated labels
	Labels *string `json:"labels,omitempty"`
	// File is the Base64 encoded file to create a document from
	File *string `json:"file,omitempty"`
	// FileName is the name of the file, required if the file is provided
	FileName *string `json:"file_name,omitempty"`
	// SourceUrl is the source URL for the document
	SourceUrl *string `json:"source_url,omitempty"`
	// Text is the text to create a document from
	Text *string `json:"text,omitempty"`
}

// BulkResponse is the paginated envelope returned by GetDocuments
// and related bulk endpoints.
type BulkResponse struct {
	Results    []Document `json:"results,omitempty"`
	Page       int        `json:"page,omitempty"`
	TotalCount int        `json:"total_count,omitempty"`
	NextPage   string     `json:"next_page,omitempty"`
	PrevPage   string     `json:"prev_page,omitempty"`
}

// Document is Geni's Document resource — uploaded files, text records,
// or external source URLs attached to profiles.
type Document struct {
	// ID is the document's id
	ID string `json:"id,omitempty"`
	// Title is the document's title
	Title string `json:"title,omitempty"`
	// Description is the document's description
	Description *string `json:"description"`
	// SourceUrl is the document's source URL
	SourceUrl *string `json:"source_url"`
	// ContentType is the document's content type
	ContentType *string `json:"content_type"`
	// Date is the document's date
	Date *profile.DateElement `json:"date"`
	// Location is the document's location
	Location *profile.LocationElement `json:"location,omitempty"`
	// Profiles is the list of profiles tagged in the document
	Tags []string `json:"tags"`
	// Labels is the list of labels associated with the document
	Labels []string `json:"labels"`
	// UpdatedAt is the timestamp of when the document was last updated
	UpdatedAt string `json:"updated_at"`
	// CreatedAt is the timestamp of when the document was created
	CreatedAt string `json:"created_at"`
}
