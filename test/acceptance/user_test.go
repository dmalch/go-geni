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
			// account_type is documented as one of basic / plus / pro;
			// the sandbox test account should fall in that set.
			Expect(user.AccountType).To(BeElementOf("basic", "plus", "pro"))
			// Name is the documented user-facing display name; assert
			// non-empty rather than a specific value (test accounts
			// vary between sandbox provisioning runs).
			Expect(user.Name).ToNot(BeEmpty())
		})
	})
})
