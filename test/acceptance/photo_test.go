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

			photo, err := client.CreatePhoto(ctx, title, "acc.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			Expect(photo.Id).To(HavePrefix("photo-"))
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			Expect(photo.Title).To(Equal(title))
		})
	})

	Describe("GetPhoto", func() {
		It("reads back a freshly-uploaded photo", func() {
			title := fmt.Sprintf("AccGetPhoto-%d", time.Now().UnixNano())
			photo, err := client.CreatePhoto(ctx, title, "get.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			got, err := client.GetPhoto(ctx, photo.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(photo.Id))
			Expect(got.Title).To(Equal(title))
		})
	})

	Describe("DeletePhoto", func() {
		// Geni may soft-delete photos (a follow-up GET still succeeds
		// like it does for documents). We only assert the delete call
		// itself returns no error or maps a stale id to
		// ErrResourceNotFound on a second delete.
		It("removes a freshly-uploaded photo", func() {
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccDeletePhoto-%d", time.Now().UnixNano()),
				"del.png",
				tinyPng())
			Expect(err).ToNot(HaveOccurred())

			Expect(client.DeletePhoto(ctx, photo.Id)).To(Succeed())

			// Deleting twice should either succeed (sandbox no-op) or
			// surface ErrResourceNotFound — both are acceptable.
			err = client.DeletePhoto(ctx, photo.Id)
			if err != nil {
				Expect(errors.Is(err, geni.ErrResourceNotFound)).To(BeTrue(),
					"unexpected error on double-delete: %v", err)
			}
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

			photo, err := client.CreatePhoto(ctx, title, "opts.png", tinyPng(),
				geni.WithPhotoDescription(description))
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			got, err := client.GetPhoto(ctx, photo.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(photo.Id))
			Expect(got.Description).To(Equal(description))
		})
	})

	Describe("UpdatePhoto", func() {
		It("updates a photo's title + description and the change round-trips", func() {
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccUpdatePhotoBefore-%d", time.Now().UnixNano()),
				"upd.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			newTitle := "AccUpdatePhotoAfter"
			updated, err := client.UpdatePhoto(ctx, photo.Id, &geni.PhotoRequest{
				Title:       newTitle,
				Description: "edited by acceptance",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Title).To(Equal(newTitle))
			Expect(updated.Description).To(Equal("edited by acceptance"))

			got, err := client.GetPhoto(ctx, photo.Id)
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
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccPhotoTag-%d", time.Now().UnixNano()),
				"tag.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			// TagPhoto's response body sometimes returns the photo
			// with an empty Tags slice even after a successful tag —
			// the authoritative read is GetPhotoTags.
			_, err = client.TagPhoto(ctx, photo.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			tags, err := client.GetPhotoTags(ctx, photo.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			ids := make([]string, 0, len(tags.Results))
			for _, p := range tags.Results {
				ids = append(ids, p.Id)
			}
			Expect(ids).To(ContainElement(profile.Id))

			_, err = client.UntagPhoto(ctx, photo.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())

			tagsAfter, err := client.GetPhotoTags(ctx, photo.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			idsAfter := make([]string, 0, len(tagsAfter.Results))
			for _, p := range tagsAfter.Results {
				idsAfter = append(idsAfter, p.Id)
			}
			Expect(idsAfter).ToNot(ContainElement(profile.Id))
		})
	})

	Describe("Comments", func() {
		// Mirrors the document.comment sandbox finding: the wire
		// calls succeed but the comment may not surface in the list
		// envelope. We assert the call shapes only.
		It("posts a comment and lists comments without error", func() {
			photo, err := client.CreatePhoto(ctx,
				fmt.Sprintf("AccPhotoComment-%d", time.Now().UnixNano()),
				"cmt.png", tinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeletePhoto(context.Background(), photo.Id) })

			added, err := client.AddPhotoComment(ctx, photo.Id, "first photo comment", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(added).ToNot(BeNil())

			listed, err := client.GetPhotoComments(ctx, photo.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(listed).ToNot(BeNil())
		})
	})
})
