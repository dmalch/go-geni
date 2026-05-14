package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// inlineTinyPng generates a 1×1 PNG so the photos spec doesn't need a
// binary fixture. Mirrors tinyPng in photo_test.go — duplicated here
// to keep this file standalone.
func inlineTinyPng() io.Reader {
	GinkgoHelper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	Expect(png.Encode(&buf, img)).To(Succeed())
	return &buf
}

var _ = Describe("Profile media listings", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetProfileDocuments", func() {
		// Sandbox finding: tagging a document with a profile
		// (TagDocument) succeeds but the document doesn't surface on
		// GetProfileDocuments(profile) immediately — propagation is
		// async or filtered. We tag and call to confirm the wire
		// path works; membership of the just-tagged document in the
		// returned page is not assertable in the sandbox.
		It("returns a valid documents envelope for the profile", func() {
			profile := createFixtureProfile(ctx, client, "ProfileDocs")
			doc := createFixtureDocument(ctx, client,
				fmt.Sprintf("AccProfileDocsDoc-%d", time.Now().UnixNano()),
				"profile-listing fixture")

			_, err := client.TagDocument(ctx, doc.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			listed, err := client.GetProfileDocuments(ctx, profile.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(listed).ToNot(BeNil())
		})
	})

	Describe("GetProfilePhotos", func() {
		// Same propagation lag as GetProfileDocuments — TagPhoto
		// succeeds, GetPhotoTags eventually reflects it, but
		// GetProfilePhotos(profile) returns an empty results array
		// in the sandbox immediately after.
		It("returns a valid photos envelope for the profile", func() {
			profile := createFixtureProfile(ctx, client, "ProfilePhotos")
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccProfilePhotosPhoto-%d", time.Now().UnixNano()),
				"plist.png", inlineTinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			_, err = client.TagPhoto(ctx, photo.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			listed, err := client.GetProfilePhotos(ctx, profile.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(listed).ToNot(BeNil())
		})
	})
})
