package acceptance

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/user"
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
			user, err := client.User().Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(user).ToNot(BeNil())

			// Log everything the real API surfaces so deviations from
			// the documented fields are visible in test output.
			AddReportEntry("user.ID", user.ID)
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
			first, err := client.User().Get(ctx)
			Expect(err).ToNot(HaveOccurred())

			second, err := client.User().Get(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(second.Name).To(Equal(first.Name))
			Expect(second.AccountType).To(Equal(first.AccountType))
			// If one call surfaces an ID, the next must surface the
			// same one (or both must omit it).
			Expect(second.ID).To(Equal(first.ID))
			Expect(second.Guid).To(Equal(first.Guid))
		})
	})

	Describe("Add", func() {
		// /user/add creates a real account, and Geni's API has no
		// user-delete endpoint — the account cannot be cleaned up.
		// The spec is therefore opt-in: set GENI_E2E_RUN_USER_ADD=1
		// to actually run it.
		It("creates a new user and surfaces its OAuth token", func() {
			if os.Getenv("GENI_E2E_RUN_USER_ADD") != "1" {
				Skip("set GENI_E2E_RUN_USER_ADD=1 to run /user/add — it creates a permanent, non-deletable sandbox account")
			}

			email := fmt.Sprintf("go-geni-e2e-%d@example.com", time.Now().UnixNano())
			res, err := client.User().Add(ctx, &user.AddRequest{
				Email:     email,
				FirstName: "GoGeni",
				LastName:  "E2E",
				Gender:    "u",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.User).ToNot(BeNil())
			Expect(res.AccessToken).ToNot(BeEmpty())

			AddReportEntry("new user.Name", res.User.Name)
			AddReportEntry("new user.AccountType", res.User.AccountType)
		})
	})
})
