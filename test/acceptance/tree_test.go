package acceptance

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("Tree traversal API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetImmediateFamily", func() {
		// Sandbox state for freshly-grafted profiles is not stable
		// enough to assert on Nodes content within a single spec —
		// we cover that path in the unit/Ginkgo httptest tiers.
		It("echoes the focus profile id", func() {
			focus := createFixtureProfile(ctx, client, "FocusFamily")

			family, err := client.GetImmediateFamily(ctx, focus.Id)

			Expect(err).ToNot(HaveOccurred())
			Expect(family.Focus).ToNot(BeNil())
			Expect(family.Focus.Id).To(Equal(focus.Id))
		})
	})

	Describe("GetAncestors", func() {
		// Geni's sandbox returns 403 for ancestors of freshly-created
		// isolated profiles, even with the `family` OAuth scope on
		// the token. The endpoint may require the focus profile to
		// already be attached to the calling user's verified tree —
		// the public docs don't say. Skip rather than fail when we
		// hit that path; the request wiring is covered by the unit
		// and httptest tiers.
		It("echoes the focus profile id when authorized", func() {
			root := createFixtureProfile(ctx, client, "AncestorRoot")

			ancestors, err := client.GetAncestors(ctx, root.Id, geni.WithGenerations(2))
			if errors.Is(err, geni.ErrAccessDenied) {
				Skip("sandbox returned 403 on ancestors for a fresh isolated profile (Geni-side restriction, not a client bug)")
			}

			Expect(err).ToNot(HaveOccurred())
			Expect(ancestors.Focus).ToNot(BeNil())
			Expect(ancestors.Focus.Id).To(Equal(root.Id))
		})
	})

	Describe("GetPathTo", func() {
		// The notify/email suppression options keep the call
		// side-effect-free. Geni may return Pending on first call;
		// we don't poll — we just assert a recognized PathStatus
		// comes back.
		It("returns a recognized PathStatus for a parent→child path", func() {
			parent := createFixtureProfile(ctx, client, "PathToParent")

			child, err := client.AddChild(ctx, parent.Id)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			res, err := client.GetPathTo(ctx, parent.Id, child.Id,
				geni.WithSkipEmail(true),
				geni.WithSkipNotify(true),
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(BeElementOf(
				geni.PathStatusDone,
				geni.PathStatusPending,
				geni.PathStatusOverloaded,
				geni.PathStatusNotFound,
			))
		})
	})
})
