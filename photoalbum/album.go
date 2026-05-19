// Package photoalbum carries the PhotoAlbum resource's wire types.
// Photo Album API methods still live on github.com/dmalch/go-geni's
// root Client during the pre-1.0 reshape and migrate into this
// package in a later PR; this PR lifts only the types.
package photoalbum

// PhotoAlbum is Geni's PhotoAlbum resource — a container for related
// photos. Returned by GetMyAlbums, CreatePhotoAlbum, GetPhotoAlbum,
// and UpdatePhotoAlbum.
type PhotoAlbum struct {
	Id          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	// CoverPhoto is a size-keyed map of cover-image URLs, same shape
	// as photo.Photo.Sizes.
	CoverPhoto map[string]string `json:"cover_photo,omitempty"`
	// PhotosCount is the number of photos in the album.
	PhotosCount int    `json:"photos_count,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// Request is the JSON-encoded body for CreatePhotoAlbum and
// UpdatePhotoAlbum. Geni's public docs don't enumerate the exact
// accepted fields for add / update — the conventional pair below
// (name + description) is what the resource model exposes.
type Request struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// BulkResponse is the paginated envelope returned by GetMyAlbums.
type BulkResponse struct {
	Results  []PhotoAlbum `json:"results,omitempty"`
	Page     int          `json:"page,omitempty"`
	NextPage string       `json:"next_page,omitempty"`
	PrevPage string       `json:"prev_page,omitempty"`
}
