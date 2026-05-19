package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("Stats / Revision API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetStats", func() {
		It("returns the platform's stats list", func() {
			res, err := client.Stats().Get(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			// The sandbox should always have at least one stat;
			// we don't assert specific names since they're opaque.
			Expect(res.Stats).ToNot(BeEmpty())
		})
	})

	Describe("GetRevision", func() {
		// Skipped: revision ids are produced by Geni's edit history
		// and aren't directly queryable from the public API for
		// arbitrary profiles. The wire-shape coverage lives in the
		// unit and integration tiers.
		It("reads a known revision", func() {
			Skip("requires a known sandbox revision id — set GENI_E2E_REVISION_ID and remove this Skip to enable")
		})
	})
})
