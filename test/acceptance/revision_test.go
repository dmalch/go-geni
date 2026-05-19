package acceptance

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Revision API", func() {
	Describe("Get", func() {
		// Skipped: revision ids are produced by Geni's edit history
		// and aren't directly queryable from the public API for
		// arbitrary profiles. The wire-shape coverage lives in the
		// unit and integration tiers.
		It("reads a known revision", func() {
			Skip("requires a known sandbox revision id — set GENI_E2E_REVISION_ID and remove this Skip to enable")
		})
	})
})
