package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("User endpoints API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("Followed listings (wire-shape)", func() {
		// These listings reflect what the calling user has chosen
		// to follow. The test account may have an empty or
		// non-empty set; we don't assert specific membership, only
		// that the call returns a valid envelope.

		It("GetFollowedProfiles returns a non-nil envelope", func() {
			res, err := client.User().FollowedProfiles(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})

		It("GetFollowedDocuments returns a non-nil envelope", func() {
			res, err := client.User().FollowedDocuments(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})

		It("GetFollowedProjects returns a non-nil envelope", func() {
			res, err := client.User().FollowedProjects(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})

		It("GetFollowedSurnames returns a non-nil envelope", func() {
			res, err := client.User().FollowedSurnames(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetMaxFamily", func() {
		It("returns a non-nil envelope for the calling user", func() {
			res, err := client.User().MaxFamily(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetUploadedPhotos / GetUploadedVideos", func() {
		// Both listings observed to return empty in the sandbox
		// even after the rest of the suite uploads dozens of
		// photos via CreatePhoto. The /user/uploaded-* endpoints
		// likely have a different visibility filter than what
		// test fixtures populate (e.g. "user-uploaded via web,
		// not via API"). We assert call shape only.

		It("GetUploadedPhotos returns a non-nil envelope", func() {
			res, err := client.User().UploadedPhotos(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})

		It("GetUploadedVideos returns a non-nil envelope", func() {
			res, err := client.User().UploadedVideos(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetMyAlbums / GetMyLabels", func() {
		It("GetMyAlbums returns a non-nil envelope", func() {
			res, err := client.User().Albums(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})

		It("GetMyLabels returns a non-nil envelope", func() {
			res, err := client.User().Labels(ctx, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("Metadata round-trip", func() {
		// UpdateMetadata + GetMetadata: write a unique payload and
		// confirm the next read surfaces it. Each test run uses a
		// fresh nonce so concurrent runs don't stomp on each
		// other.
		It("UpdateMetadata followed by GetMetadata returns the stored blob", func() {
			nonce := fmt.Sprintf(`{"nonce":%d}`, time.Now().UnixNano())
			payload := json.RawMessage(nonce)

			_, err := client.User().UpdateMetadata(ctx, payload)
			Expect(err).ToNot(HaveOccurred())

			md, err := client.User().Metadata(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(md).ToNot(BeNil())
			Expect(string(md.Data)).To(ContainSubstring(`"nonce"`))
		})
	})
})
