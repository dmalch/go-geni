// Package comment carries the Comment resource's wire types. Comment
// API methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into the relevant
// per-resource sub-packages later; this PR lifts only the types.
package comment

// Comment is Geni's Comment resource — the body of a single comment
// returned by document/photo/video comment-listing endpoints.
type Comment struct {
	// Id is the comment's identifier. Not described on the public
	// schema page but reliably present in real responses; defensively
	// captured so callers can reference individual comments.
	Id string `json:"id,omitempty"`
	// Comment is the free-text content of the comment.
	Comment string `json:"comment,omitempty"`
	// Title is the comment's optional title.
	Title string `json:"title,omitempty"`
	// CreatedAt is the comment's creation timestamp.
	CreatedAt string `json:"created_at,omitempty"`
}

// BulkResponse is the paginated envelope returned by the
// `*/comments` and `*/comment` endpoints (document.comments,
// document.comment, photo.comments, etc.).
type BulkResponse struct {
	Results  []Comment `json:"results,omitempty"`
	Page     int       `json:"page,omitempty"`
	NextPage string    `json:"next_page,omitempty"`
	PrevPage string    `json:"prev_page,omitempty"`
}
