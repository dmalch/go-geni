package acceptance

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Surname API", func() {
	Describe("Get", func() {
		// Skipped: we don't have a portable surname id — surname
		// records are auto-created by Geni from profile lastnames,
		// but the id mapping isn't directly queryable through the
		// public API. The wire-shape coverage lives in the unit
		// and integration tiers.
		It("reads a known surname", func() {
			Skip("requires a known sandbox surname id — set GENI_E2E_SURNAME_ID and remove this Skip to enable")
		})
	})
})
