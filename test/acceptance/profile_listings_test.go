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

// How long we'll wait for cross-resource state to propagate to the
// profile's documents / photos listing once the test fixture path
// actually populates it. Currently unused — the specs that use these
// are Skip()'d (see comments inline) — but kept so the eventual
// assertion is realistic when the missing endpoints land.
const (
	profileListingPropagationTimeout = 60 * time.Second
	profileListingPropagationPoll    = 5 * time.Second
)

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
		// Skipped: the sandbox listing reflects documents the
		// profile *owns* (via profile/add-document — not yet
		// implemented in this client), not documents tagged on the
		// profile via document.tag. Running this spec against the
		// current sandbox times out at 60s with the document never
		// surfacing. Unskip once profile/add-document lands and
		// drive the setup through it instead of TagDocument.
		It("eventually lists a document attached to the profile", func() {
			Skip("requires profile/add-document endpoint — TagDocument doesn't propagate to /profile/{id}/documents in the sandbox")

			profile := createFixtureProfile(ctx, client, "ProfileDocs")
			doc := createFixtureDocument(ctx, client,
				fmt.Sprintf("AccProfileDocsDoc-%d", time.Now().UnixNano()),
				"profile-listing fixture")

			_, err := client.TagDocument(ctx, doc.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				listed, err := client.GetProfileDocuments(ctx, profile.Id, 0)
				g.Expect(err).ToNot(HaveOccurred())
				ids := make([]string, 0, len(listed.Results))
				for _, d := range listed.Results {
					ids = append(ids, d.Id)
				}
				g.Expect(ids).To(ContainElement(doc.Id))
			}).
				WithTimeout(profileListingPropagationTimeout).
				WithPolling(profileListingPropagationPoll).
				Should(Succeed())
		})
	})

	Describe("GetProfilePhotos", func() {
		// Skipped for the same reason as GetProfileDocuments:
		// requires profile/add-photo (not yet implemented) to
		// produce a photo the listing actually reflects. TagPhoto
		// alone doesn't propagate.
		It("eventually lists a photo attached to the profile", func() {
			Skip("requires profile/add-photo endpoint — TagPhoto doesn't propagate to /profile/{id}/photos in the sandbox")

			profile := createFixtureProfile(ctx, client, "ProfilePhotos")
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccProfilePhotosPhoto-%d", time.Now().UnixNano()),
				"plist.png", inlineTinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			_, err = client.TagPhoto(ctx, photo.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				listed, err := client.GetProfilePhotos(ctx, profile.Id, 0)
				g.Expect(err).ToNot(HaveOccurred())
				ids := make([]string, 0, len(listed.Results))
				for _, p := range listed.Results {
					ids = append(ids, p.Id)
				}
				g.Expect(ids).To(ContainElement(photo.Id))
			}).
				WithTimeout(profileListingPropagationTimeout).
				WithPolling(profileListingPropagationPoll).
				Should(Succeed())
		})
	})
})
