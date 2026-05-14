package acceptance

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// These specs probe the Geni sandbox to localise the 403 behaviour
// on /ancestors. They're not regressions — they're an experiment log
// that runs against the live sandbox and reports which call shapes
// the server actually accepts. Read the Ginkgo output to compare.

var _ = Describe("Drill: GetAncestors access rules", Label("drill"), func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	// Baseline: a freshly-created isolated profile reliably 403s on
	// ancestors. Document it here so deviations are visible.
	It("freshly-created profile → ancestors → 403 (baseline)", func() {
		root := createFixtureProfile(ctx, client, "DrillFresh")
		_, err := client.GetAncestors(ctx, root.Id, geni.WithGenerations(2))
		AddReportEntry("fresh-profile-ancestors-err", fmt.Sprintf("%v", err))
		Expect(errors.Is(err, geni.ErrAccessDenied)).To(BeTrue(),
			"if this stops being 403, the access rule has changed; got %v", err)
	})

	// Control: immediate-family on the same fresh profile works.
	// Pins down that the 403 is endpoint-specific, not blanket
	// "isolated profile is unreadable".
	It("freshly-created profile → immediate-family → ok (control)", func() {
		root := createFixtureProfile(ctx, client, "DrillFreshControl")
		fam, err := client.GetImmediateFamily(ctx, root.Id)
		Expect(err).ToNot(HaveOccurred())
		Expect(fam.Focus).ToNot(BeNil())
		Expect(fam.Focus.Id).To(Equal(root.Id))
	})

	// "me" alias: Geni's docs sometimes accept `me` as a profile
	// alias, but for this endpoint the server replies with a 500
	// ActionController "No action responded to me" — `me` isn't a
	// valid path param here. Recorded so a future docs/sandbox
	// change is easy to spot.
	It("alias `me` → ancestors → 500 (not a valid path)", func() {
		_, err := client.GetAncestors(ctx, "me", geni.WithGenerations(2))
		AddReportEntry("me-ancestors-err", fmt.Sprintf("%v", err))
		Expect(err).To(HaveOccurred(), "if `me` ever starts resolving, update this spec")
	})

	// A profile that the test account already manages should be
	// "verified-tree-resident" by definition. If ancestors accepts
	// these but rejects freshly-created ones, the access rule is
	// "focus must be in the caller's managed tree".
	It("managed-profiles[0] → ancestors", func() {
		managed, err := client.GetManagedProfiles(ctx, 1)
		Expect(err).ToNot(HaveOccurred())
		Expect(managed.Results).ToNot(BeEmpty(), "GetManagedProfiles returned no results; can't probe")

		// Pick the first non-deleted result. Skip deleted shells.
		var target *geni.ProfileResponse
		for i := range managed.Results {
			if !managed.Results[i].Deleted {
				target = &managed.Results[i]
				break
			}
		}
		Expect(target).ToNot(BeNil(), "no live managed profile to probe")
		AddReportEntry("managed-target-id", target.Id)

		fam, err := client.GetAncestors(ctx, target.Id, geni.WithGenerations(2))
		AddReportEntry("managed-ancestors-err", fmt.Sprintf("%v", err))
		if errors.Is(err, geni.ErrAccessDenied) {
			Skip("ancestors on a managed profile was also denied — Geni's rule is stricter than 'managed' or the scope is wrong")
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(fam.Focus).ToNot(BeNil())
		Expect(fam.Focus.Id).To(Equal(target.Id))
	})

	// Build a 3-generation chain (gp → p → me) with profile.add-parent
	// — wait, that endpoint isn't in the client. Use AddChild from a
	// root downward instead and call ancestors on the youngest. If
	// ancestors works on a profile that has explicit ancestors in
	// the caller's tree, but not on a leaf with none, the rule is
	// "must have at least one resolvable ancestor in the tree".
	It("created chain root→child → ancestors(child)", func() {
		gp := createFixtureProfile(ctx, client, "DrillGrandparent")
		parent, err := client.AddChild(ctx, gp.Id)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), parent.Id) })

		child, err := client.AddChild(ctx, parent.Id)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

		fam, err := client.GetAncestors(ctx, child.Id, geni.WithGenerations(3))
		AddReportEntry("chain-child-ancestors-err", fmt.Sprintf("%v", err))
		if errors.Is(err, geni.ErrAccessDenied) {
			Skip("ancestors on a hand-built created chain was also denied")
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(fam.Focus).ToNot(BeNil())
		Expect(fam.Focus.Id).To(Equal(child.Id))
	})
})
