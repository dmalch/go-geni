package acceptance

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("Photo Album API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	// createFixtureAlbum creates an album in the sandbox and
	// registers a no-op cleanup (the photo_album resource has no
	// documented delete endpoint). Sandbox state accumulates;
	// using high-entropy names so test runs don't collide.
	createFixtureAlbum := func() *geni.PhotoAlbum {
		GinkgoHelper()
		desc := "sandbox-test"
		album, err := client.CreatePhotoAlbum(ctx, &geni.PhotoAlbumRequest{
			Name:        fmt.Sprintf("AccAlbum-%d", time.Now().UnixNano()),
			Description: &desc,
		})
		Expect(err).ToNot(HaveOccurred())
		return album
	}

	Describe("CreatePhotoAlbum", func() {
		It("creates a new album and returns its id", func() {
			album := createFixtureAlbum()
			Expect(album.Id).To(HavePrefix("album-"))
			Expect(album.Name).To(HavePrefix("AccAlbum-"))
		})
	})

	Describe("GetPhotoAlbum", func() {
		It("reads a freshly-created album back", func() {
			created := createFixtureAlbum()

			got, err := client.GetPhotoAlbum(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(created.Id))
			Expect(got.Name).To(Equal(created.Name))
		})
	})

	Describe("UpdatePhotoAlbum", func() {
		It("renames an album and the change round-trips", func() {
			created := createFixtureAlbum()
			newName := fmt.Sprintf("AccAlbumRenamed-%d", time.Now().UnixNano())

			updated, err := client.UpdatePhotoAlbum(ctx, created.Id, &geni.PhotoAlbumRequest{
				Name: newName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Id).To(Equal(created.Id))
			Expect(updated.Name).To(Equal(newName))

			got, err := client.GetPhotoAlbum(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Name).To(Equal(newName))
		})
	})

	Describe("GetPhotoAlbumPhotos", func() {
		// Wire-shape check only. Linking photos to an album via
		// CreatePhoto + WithPhotoAlbum is a separate concern —
		// when last tested, Geni's response returned a numeric
		// album_id (e.g. "951529") rather than the prefixed
		// album id ("album-44"), so direct membership assertions
		// would race against an id-format mismatch.
		It("returns a non-nil envelope for a fresh album", func() {
			created := createFixtureAlbum()

			res, err := client.GetPhotoAlbumPhotos(ctx, created.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})
})
