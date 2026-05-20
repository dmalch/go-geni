// Package photo carries the Photo resource's wire types. Photo API
// methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into this package in a
// later PR; this PR lifts only the types.
package photo

import "github.com/dmalch/go-geni/profile"

// Request is the JSON-encoded body for UpdatePhoto and the
// JSON-body /profile/{id}/add-photo path. All fields are optional;
// omitted fields leave the existing value in place (for Update) or
// aren't sent at all.
//
// File is the Base64-encoded image content. It's only meaningful on
// the add-photo path — the multipart /photo/add endpoint uses a
// streaming io.Reader instead.
type Request struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Date        string  `json:"date,omitempty"`
	File        *string `json:"file,omitempty"`
}

// Photo is Geni's Photo resource — a single uploaded image with
// metadata and tagging.
type Photo struct {
	// Id is the photo's identifier.
	Id string `json:"id,omitempty"`
	// Guid is the photo's legacy global identifier.
	Guid string `json:"guid,omitempty"`
	// AlbumId is the id of the album containing the photo.
	AlbumId string `json:"album_id,omitempty"`
	// Title is the photo's title.
	Title string `json:"title,omitempty"`
	// Description is the photo's description.
	Description string `json:"description,omitempty"`
	// Date is the photo's date, as returned by Geni (string format).
	Date string `json:"date,omitempty"`
	// Attribution is the photo's attribution string.
	Attribution string `json:"attribution,omitempty"`
	// ContentType is the original MIME type of the upload.
	ContentType string `json:"content_type,omitempty"`
	// Location is the photo's optional location.
	Location *profile.LocationElement `json:"location,omitempty"`
	// Tags is the list of profiles tagged in the photo (urls or ids
	// depending on the `only_ids` query parameter).
	Tags []string `json:"tags,omitempty"`
	// Sizes maps Geni-defined size names (e.g. "small", "medium",
	// "large") to fully-qualified image URLs. The exact set of keys
	// varies by upload.
	Sizes map[string]string `json:"sizes,omitempty"`
	// Url is the API URL for the photo.
	Url string `json:"url,omitempty"`
	// CreatedAt / UpdatedAt are the resource lifecycle timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// BulkResponse is the envelope returned by GetPhotos and the
// paginated `*/photos` listings (profile.photos, album.photos, etc.).
type BulkResponse struct {
	Results  []Photo `json:"results,omitempty"`
	Page     int     `json:"page,omitempty"`
	NextPage string  `json:"next_page,omitempty"`
	PrevPage string  `json:"prev_page,omitempty"`
}

// MugshotRequest is the JSON-encoded body for
// [Client.AddMugshotToProfile]. Either File or PhotoId is required —
// File uploads a new image via Base64 (the JSON path; not the
// multipart one), PhotoId reuses an existing photo as the mugshot.
type MugshotRequest struct {
	// File is the Base64-encoded image to upload as the mugshot.
	// Mutually exclusive with PhotoId; required when PhotoId is not
	// set.
	File *string `json:"file,omitempty"`
	// PhotoId reuses an existing photo as the mugshot. Mutually
	// exclusive with File; required when File is not set.
	PhotoId *string `json:"photo_id,omitempty"`
	// Title, Description, Date, AlbumId are all optional.
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Date        *string `json:"date,omitempty"`
	AlbumId     *string `json:"album_id,omitempty"`
}
