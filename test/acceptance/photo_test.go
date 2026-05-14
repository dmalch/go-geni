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
})
