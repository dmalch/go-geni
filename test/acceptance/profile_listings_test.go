package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/photo"
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
// has actually populated it via the profile/add-* endpoints
// introduced in v0.14.0. Before that, the only setup primitives
// were TagDocument / TagPhoto, which Geni doesn't propagate to
// these listings — those specs were Skip()'d.
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
		// Skipped: even with the now-supported AddProfileDocument
		// (v0.14.0) — the "right" owns-this-document verb — the
		// sandbox listing still doesn't reflect the new document
		// within a 60s polling window. Whatever filter or
		// propagation mechanism the listing uses, neither
		// TagDocument nor AddProfileDocument triggers it on the
		// timescale tests can realistically wait for. The
		// AddProfileDocument call itself is verified in
		// profile_actions_test.go; this spec stays as the
		// canonical regression target should sandbox propagation
		// ever speed up.
		It("eventually lists a document attached via AddProfileDocument", func() {
			Skip("/profile/{id}/documents doesn't reflect new documents within 60s, even after AddProfileDocument")

			profile := createFixtureProfile(ctx, client, "ProfileDocs")
			text := "profile-listing fixture"

			doc, err := client.AddProfileDocument(ctx, profile.Id, &document.Request{
				Title: fmt.Sprintf("AccProfileDocsDoc-%d", time.Now().UnixNano()),
				Text:  &text,
			})
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteDocument(context.Background(), doc.Id) })

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
		// Same finding as the documents listing: AddProfilePhoto
		// succeeds (covered in profile_actions_test.go) but the
		// /profile/{id}/photos listing doesn't reflect it within
		// 60s.
		It("eventually lists a photo attached via AddProfilePhoto", func() {
			Skip("/profile/{id}/photos doesn't reflect new photos within 60s, even after AddProfilePhoto")

			profile := createFixtureProfile(ctx, client, "ProfilePhotos")

			// inlineTinyPng yields raw PNG bytes; the JSON-body
			// endpoint expects Base64.
			raw, err := io.ReadAll(inlineTinyPng())
			Expect(err).ToNot(HaveOccurred())
			b64 := base64.StdEncoding.EncodeToString(raw)

			photo, err := client.AddProfilePhoto(ctx, profile.Id, &photo.Request{
				Title: fmt.Sprintf("AccProfilePhotosPhoto-%d", time.Now().UnixNano()),
				File:  &b64,
			})
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

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
