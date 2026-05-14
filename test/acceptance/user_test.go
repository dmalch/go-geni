package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("User API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetUser", func() {
		It("returns the authenticated user's account info", func() {
			user, err := client.GetUser(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(user).ToNot(BeNil())

			// Log everything the real API surfaces so deviations from
			// the documented fields are visible in test output.
			AddReportEntry("user.Id", user.Id)
			AddReportEntry("user.Guid", user.Guid)
			AddReportEntry("user.Name", user.Name)
			AddReportEntry("user.AccountType", user.AccountType)

			// account_type is documented as one of basic / plus / pro.
			Expect(user.AccountType).To(BeElementOf("basic", "plus", "pro"))
			// Name is the documented user-facing display name.
			Expect(user.Name).ToNot(BeEmpty())
		})

		It("returns a stable identity across calls", func() {
			// Idempotency check: two back-to-back calls should
			// describe the same account. Catches regressions where
			// the OAuth flow gets crossed mid-suite or the response
			// envelope changes shape between requests.
			first, err := client.GetUser(ctx)
			Expect(err).ToNot(HaveOccurred())

			second, err := client.GetUser(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(second.Name).To(Equal(first.Name))
			Expect(second.AccountType).To(Equal(first.AccountType))
			// If one call surfaces an Id, the next must surface the
			// same one (or both must omit it).
			Expect(second.Id).To(Equal(first.Id))
			Expect(second.Guid).To(Equal(first.Guid))
		})
	})
})
