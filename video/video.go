// Package video carries the Video resource's wire types. Video API
// methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into this package in a
// later PR; this PR lifts only the types.
package video

import "github.com/dmalch/go-geni/profile"

// Request is the JSON-encoded body for UpdateVideo and the
// JSON-body /profile/{id}/add-video path. File is the Base64-encoded
// video content for the add-video path only — the /video/add
// multipart endpoint uses a streaming io.Reader instead.
type Request struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Date        string  `json:"date,omitempty"`
	File        *string `json:"file,omitempty"`
}

// Video is Geni's Video resource — a single uploaded video (or a
// link to an externally-hosted video) with metadata and tagging.
type Video struct {
	// Id is the video's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the video's legacy global identifier.
	Guid string `json:"guid,omitempty"`
	// Title is the video's title.
	Title string `json:"title,omitempty"`
	// Description is the video's description.
	Description string `json:"description,omitempty"`
	// Date is the video's date, as returned by Geni (string format).
	Date string `json:"date,omitempty"`
	// Attribution is the video's attribution string.
	Attribution string `json:"attribution,omitempty"`
	// ContentType is the original MIME type of the upload.
	ContentType string `json:"content_type,omitempty"`
	// Location is the video's optional location.
	Location *profile.LocationElement `json:"location,omitempty"`
	// Tags is the list of profiles tagged in the video (urls or ids
	// depending on the `only_ids` query parameter).
	Tags []string `json:"tags,omitempty"`
	// Sizes maps Geni-defined size names to fully-qualified URLs.
	Sizes map[string]string `json:"sizes,omitempty"`
	// Url is the API URL for the video.
	Url string `json:"url,omitempty"`
	// CreatedAt / UpdatedAt are the resource lifecycle timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// BulkResponse is the envelope returned by GetVideos.
type BulkResponse struct {
	Results []Video `json:"results,omitempty"`
}
