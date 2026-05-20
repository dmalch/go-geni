// Package geni is a Go client for the Geni.com genealogy API.
//
// geni.Client is a thin façade: it constructs one shared HTTP
// transport and exposes a per-resource client for each of Geni's
// resources via an accessor method (Profile, Union, Document, Photo,
// Video, PhotoAlbum, Project, Surname, Revision, Stats, User, Search,
// Tree). Each accessor returns a *Client from the matching
// sub-package — e.g. Client.Profile() returns a *profile.Client.
//
// Callers that only need one resource can import that sub-package
// directly and construct its client from a *transport.Client; the
// façade is an ergonomic aggregate, not a required entry point.
package geni

import (
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/photoalbum"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/project"
	"github.com/dmalch/go-geni/revision"
	"github.com/dmalch/go-geni/search"
	"github.com/dmalch/go-geni/stats"
	"github.com/dmalch/go-geni/surname"
	"github.com/dmalch/go-geni/transport"
	"github.com/dmalch/go-geni/tree"
	"github.com/dmalch/go-geni/union"
	"github.com/dmalch/go-geni/user"
	"github.com/dmalch/go-geni/video"
)

// ErrResourceNotFound is returned for 404 responses from the Geni API.
// Re-exported from the transport package so callers can errors.Is
// against geni.ErrResourceNotFound without importing transport.
var ErrResourceNotFound = transport.ErrResourceNotFound

// ErrAccessDenied is returned for 403 responses from the Geni API.
// Re-exported from the transport package.
var ErrAccessDenied = transport.ErrAccessDenied

// Client is the Geni API façade. It owns one shared transport and one
// client per resource; reach a resource through its accessor (e.g.
// [Client.Profile]).
type Client struct {
	transport  *transport.Client
	profile    *profile.Client
	union      *union.Client
	document   *document.Client
	photo      *photo.Client
	video      *video.Client
	photoalbum *photoalbum.Client
	project    *project.Client
	surname    *surname.Client
	revision   *revision.Client
	stats      *stats.Client
	user       *user.Client
	search     *search.Client
	tree       *tree.Client
}

// NewClient constructs a Client. useSandboxEnv selects between
// sandbox.geni.com (true) and www.geni.com (false).
func NewClient(tokenSource oauth2.TokenSource, useSandboxEnv bool) *Client {
	t := transport.New(tokenSource, useSandboxEnv)
	return &Client{
		transport:  t,
		profile:    profile.NewClient(t),
		union:      union.NewClient(t),
		document:   document.NewClient(t),
		photo:      photo.NewClient(t),
		video:      video.NewClient(t),
		photoalbum: photoalbum.NewClient(t),
		project:    project.NewClient(t),
		surname:    surname.NewClient(t),
		revision:   revision.NewClient(t),
		stats:      stats.NewClient(t),
		user:       user.NewClient(t),
		search:     search.NewClient(t),
		tree:       tree.NewClient(t),
	}
}

// Profile returns the client for the Profile resource — CRUD,
// relationship adds (partner / child / sibling / parent), merge,
// follow / unfollow, basics-update, and event-date wiping.
func (c *Client) Profile() *profile.Client { return c.profile }

// Union returns the client for the Union resource.
func (c *Client) Union() *union.Client { return c.union }

// Document returns the client for the Document resource — including
// the profile-scoped ForProfile / AddToProfile listings and the
// project-scoped AddToProject.
func (c *Client) Document() *document.Client { return c.document }

// Photo returns the client for the Photo resource — including the
// profile-scoped ForProfile / AddToProfile / AddMugshotToProfile
// operations.
func (c *Client) Photo() *photo.Client { return c.photo }

// Video returns the client for the Video resource — including the
// profile-scoped AddToProfile operation.
func (c *Client) Video() *video.Client { return c.video }

// PhotoAlbum returns the client for the PhotoAlbum resource.
func (c *Client) PhotoAlbum() *photoalbum.Client { return c.photoalbum }

// Project returns the client for the Project resource.
func (c *Client) Project() *project.Client { return c.project }

// Surname returns the client for the Surname resource.
func (c *Client) Surname() *surname.Client { return c.surname }

// Revision returns the client for the Revision resource.
func (c *Client) Revision() *revision.Client { return c.revision }

// Stats returns the client for the platform-wide statistics endpoint.
func (c *Client) Stats() *stats.Client { return c.stats }

// User returns the client for the User resource and all /user/*
// listings (followed-*, uploaded-*, managed-profiles, max-family,
// my-albums, my-labels, metadata).
func (c *Client) User() *user.Client { return c.user }

// Search returns the client for /profile/search.
func (c *Client) Search() *search.Client { return c.search }

// Tree returns the client for Geni's family-graph endpoints —
// immediate-family, ancestors, path-to, and compare.
func (c *Client) Tree() *tree.Client { return c.tree }

// BaseURL returns the prod or sandbox HTTP host (with trailing slash).
func BaseURL(useSandboxEnv bool) string {
	return transport.BaseURL(useSandboxEnv)
}
