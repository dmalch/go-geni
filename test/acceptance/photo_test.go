package acceptance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	photopkg "github.com/dmalch/go-geni/photo"
)

// tinyPng returns a 1×1 black PNG generated in-process so we don't
// have to ship a binary fixture.
func tinyPng() io.Reader {
	GinkgoHelper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	Expect(png.Encode(&buf, img)).To(Succeed())
	return &buf
}

var _ = Describe("Photo API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("CreatePhoto", func() {
		It("uploads a tiny PNG and returns the new photo", func() {
			title := fmt.Sprintf("AccCreatePhoto-%d", time.Now().UnixNano())

			photo, err := client.Photo().Create(ctx, title, "acc.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			Expect(photo.ID).To(HavePrefix("photo-"))
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			Expect(photo.Title).To(Equal(title))
		})
	})

	Describe("GetPhoto", func() {
		It("reads back a freshly-uploaded photo", func() {
			title := fmt.Sprintf("AccGetPhoto-%d", time.Now().UnixNano())
			photo, err := client.Photo().Create(ctx, title, "get.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			got, err := client.Photo().Get(ctx, photo.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.ID).To(Equal(photo.ID))
			Expect(got.Title).To(Equal(title))
		})
	})

	Describe("DeletePhoto", func() {
		// Geni returns HTTP 200 with the empty bulk-envelope shape
		// ({"results": []}) when a singular /api/<id> GET targets a
		// deleted photo; the transport coalescer translates that to
		// ErrResourceNotFound so callers can use the same is-deleted
		// check as for any missing resource.
		It("removes a freshly-uploaded photo and Get reports ErrResourceNotFound", func() {
			photo, err := client.Photo().Create(ctx,
				fmt.Sprintf("AccDeletePhoto-%d", time.Now().UnixNano()),
				"del.png",
				tinyPng())
			Expect(err).ToNot(HaveOccurred())

			Expect(client.Photo().Delete(ctx, photo.ID)).To(Succeed())

			_, err = client.Photo().Get(ctx, photo.ID)
			Expect(errors.Is(err, geni.ErrResourceNotFound)).To(BeTrue(),
				"expected ErrResourceNotFound after Delete, got %v", err)
		})
	})

	Describe("CreatePhoto with options", func() {
		// Verifies that an option set client-side actually reaches the
		// server and is persisted — the multipart-form fields aren't
		// merely cosmetic. We upload with a description, read the
		// photo back, and assert the description round-trips through
		// Geni's storage.
		It("persists WithPhotoDescription on the uploaded photo", func() {
			title := fmt.Sprintf("AccPhotoOpts-%d", time.Now().UnixNano())
			description := "uploaded by go-geni acceptance suite"

			photo, err := client.Photo().Create(ctx, title, "opts.png", tinyPng(),
				photopkg.WithDescription(description))
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			got, err := client.Photo().Get(ctx, photo.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.ID).To(Equal(photo.ID))
			Expect(got.Description).To(Equal(description))
		})
	})

	Describe("UpdatePhoto", func() {
		It("updates a photo's title + description and the change round-trips", func() {
			photo, err := client.Photo().Create(ctx,
				fmt.Sprintf("AccUpdatePhotoBefore-%d", time.Now().UnixNano()),
				"upd.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			newTitle := "AccUpdatePhotoAfter"
			updated, err := client.Photo().Update(ctx, photo.ID, &photopkg.Request{
				Title:       newTitle,
				Description: "edited by acceptance",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Title).To(Equal(newTitle))
			Expect(updated.Description).To(Equal("edited by acceptance"))

			got, err := client.Photo().Get(ctx, photo.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Title).To(Equal(newTitle))
		})
	})

	Describe("Tagging", func() {
		// Creates a profile and a photo, tags the profile in the
		// photo, lists the tags, then untags. Verifies the tag
		// appears in the photo's tags after Tag, and is gone from
		// the photo after Untag.
		It("tags and untags a profile in a photo", func() {
			profile := createFixtureProfile(ctx, client, "PhotoTaggee")
			photo, err := client.Photo().Create(ctx,
				fmt.Sprintf("AccPhotoTag-%d", time.Now().UnixNano()),
				"tag.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			// TagPhoto's response body sometimes returns the photo
			// with an empty Tags slice even after a successful tag —
			// the authoritative read is GetPhotoTags.
			_, err = client.Photo().Tag(ctx, photo.ID, profile.ID)
			Expect(err).ToNot(HaveOccurred())

			tags, err := client.Photo().Tags(ctx, photo.ID, 0)
			Expect(err).ToNot(HaveOccurred())
			ids := make([]string, 0, len(tags.Results))
			for _, p := range tags.Results {
				ids = append(ids, p.ID)
			}
			Expect(ids).To(ContainElement(profile.ID))

			_, err = client.Photo().Untag(ctx, photo.ID, profile.ID)
			Expect(err).ToNot(HaveOccurred())

			tagsAfter, err := client.Photo().Tags(ctx, photo.ID, 0)
			Expect(err).ToNot(HaveOccurred())
			idsAfter := make([]string, 0, len(tagsAfter.Results))
			for _, p := range tagsAfter.Results {
				idsAfter = append(idsAfter, p.ID)
			}
			Expect(idsAfter).ToNot(ContainElement(profile.ID))
		})
	})

	Describe("Comments", func() {
		// Skipped: mirrors document.Comments — AddPhotoComment
		// returns success immediately but the comment never
		// surfaces on GetPhotoComments (polled 30s). Same shape
		// as the document comments and profile media listings. The
		// intended Eventually assertion is preserved so unskipping
		// is a one-line change.
		It("posts a comment and eventually lists it back", func() {
			Skip("AddPhotoComment doesn't propagate to GetPhotoComments in the sandbox (polled 30s; comment never appeared)")

			photo, err := client.Photo().Create(ctx,
				fmt.Sprintf("AccPhotoComment-%d", time.Now().UnixNano()),
				"cmt.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })

			body := "first photo comment"
			_, err = client.Photo().AddComment(ctx, photo.ID, body, "")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				listed, err := client.Photo().Comments(ctx, photo.ID, 0)
				g.Expect(err).ToNot(HaveOccurred())
				texts := make([]string, 0, len(listed.Results))
				for _, c := range listed.Results {
					texts = append(texts, c.Comment)
				}
				g.Expect(texts).To(ContainElement(body))
			}).
				WithTimeout(30 * time.Second).
				WithPolling(2 * time.Second).
				Should(Succeed())
		})
	})
})
