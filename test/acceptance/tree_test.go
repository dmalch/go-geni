package acceptance

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// TestAccGetImmediateFamily verifies the call shape against the sandbox:
// a freshly-created profile returns a FamilyResponse whose Focus echoes
// the request. We deliberately avoid asserting on Nodes content because
// sandbox state for freshly-grafted profiles is not stable enough to
// depend on within a single test invocation.
func TestAccGetImmediateFamily(t *testing.T) {
	RegisterTestingT(t)
	ctx := context.Background()
	client := newTestClient(t)

	focus := createFixtureProfile(t, ctx, client, "FocusFamily", "Acceptance")

	family, err := client.GetImmediateFamily(ctx, focus.Id)
	Expect(err).ToNot(HaveOccurred())
	Expect(family.Focus).ToNot(BeNil())
	Expect(family.Focus.Id).To(Equal(focus.Id))
}

// TestAccGetAncestors verifies the call shape and that WithGenerations
// is honored on the wire.
//
// Note: in the sandbox the implicit-flow OAuth token frequently lacks
// the scope required to read ancestors, surfacing as a 403. When that
// happens we skip rather than fail — the regression we care about is
// "the endpoint is wired correctly", which we cover via the unit and
// httptest acceptance tiers. Promote this to a hard failure once the
// auth flow requests the right scope.
func TestAccGetAncestors(t *testing.T) {
	RegisterTestingT(t)
	ctx := context.Background()
	client := newTestClient(t)

	root := createFixtureProfile(t, ctx, client, "AncestorRoot", "Acceptance")

	ancestors, err := client.GetAncestors(ctx, root.Id, geni.WithGenerations(2))
	if errors.Is(err, geni.ErrAccessDenied) {
		t.Skipf("sandbox returned 403 on ancestors for a fresh profile (likely missing OAuth scope): %v", err)
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(ancestors.Focus).ToNot(BeNil())
	Expect(ancestors.Focus.Id).To(Equal(root.Id))
}

// TestAccGetPathTo computes the path between two related profiles. The
// notify/email suppression options keep the call side-effect-free. Geni
// may return Pending on first call; we don't poll — the test asserts
// only that a recognized PathStatus comes back.
func TestAccGetPathTo(t *testing.T) {
	RegisterTestingT(t)
	ctx := context.Background()
	client := newTestClient(t)

	parent := createFixtureProfile(t, ctx, client, "PathToParent", "Acceptance")

	child, err := client.AddChild(ctx, parent.Id)
	Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		if err := client.DeleteProfile(context.Background(), child.Id); err != nil {
			t.Logf("cleanup: delete child %s: %v", child.Id, err)
		}
	})

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
}
