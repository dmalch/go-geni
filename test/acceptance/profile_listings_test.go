package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
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
		// Wire-path check only: the freshly-created profile has no
		// documents, but the call should still return a valid
		// (empty) envelope. We don't seed a tagged document because
		// the sandbox doesn't reflect TagDocument on the profile's
		// documents listing immediately — propagation is async or
		// filtered, so seeding adds no signal.
		It("returns a valid documents envelope for a profile", func() {
			profile := createFixtureProfile(ctx, client, "ProfileDocs")

			listed, err := client.GetProfileDocuments(ctx, profile.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(listed).ToNot(BeNil())
		})
	})

	Describe("GetProfilePhotos", func() {
		// Same wire-path-only rationale as GetProfileDocuments —
		// TagPhoto doesn't surface in this listing in the sandbox.
		It("returns a valid photos envelope for a profile", func() {
			profile := createFixtureProfile(ctx, client, "ProfilePhotos")

			listed, err := client.GetProfilePhotos(ctx, profile.Id, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(listed).ToNot(BeNil())
		})
	})
})
