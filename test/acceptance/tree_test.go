package acceptance

import (
	"context"
	"errors"
	"time"

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
		// Eventually-polled: AddChild on a freshly-created profile
		// may take a few seconds for the new child to surface on
		// the parent's immediate-family graph. We poll until it
		// does. If it never does, the spec fails — which is the
		// signal we want.
		It("eventually lists a child added to the focus profile", func() {
			focus := createFixtureProfile(ctx, client, "FocusFamily")

			child, err := client.AddChild(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			Eventually(func(g Gomega) {
				family, err := client.GetImmediateFamily(ctx, focus.Id)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(family.Focus).ToNot(BeNil())
				g.Expect(family.Focus.Id).To(Equal(focus.Id))
				g.Expect(family.Nodes.ProfileIds()).To(ContainElement(child.Id))
			}).
				WithTimeout(30 * time.Second).
				WithPolling(2 * time.Second).
				Should(Succeed())
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
		// Eventually-polled: Geni's path-to endpoint is async — a
		// first call frequently returns PathStatusPending and the
		// caller is expected to re-issue. The notify/email
		// suppression options keep the call side-effect-free
		// across the polling window. We poll until status settles
		// on a non-Pending value (Done is expected for a real
		// parent→child relationship; Overloaded / NotFound are
		// also terminal).
		It("settles on a terminal PathStatus for a parent→child path", func() {
			parent := createFixtureProfile(ctx, client, "PathToParent")

			child, err := client.AddChild(ctx, parent.Id)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			var lastStatus geni.PathStatus
			Eventually(func(g Gomega) {
				res, err := client.GetPathTo(ctx, parent.Id, child.Id,
					geni.WithSkipEmail(true),
					geni.WithSkipNotify(true),
				)
				g.Expect(err).ToNot(HaveOccurred())
				lastStatus = res.Status
				g.Expect(res.Status).ToNot(Equal(geni.PathStatusPending),
					"status is still pending; keep polling")
				g.Expect(res.Status).To(BeElementOf(
					geni.PathStatusDone,
					geni.PathStatusOverloaded,
					geni.PathStatusNotFound,
				))
			}).
				WithTimeout(30*time.Second).
				WithPolling(2*time.Second).
				Should(Succeed(), "last seen status: %s", lastStatus)
		})
	})
})
