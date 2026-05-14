package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// createCoupleAndUnion creates a focus profile, adds a partner, and
// returns the focus profile, partner profile, and the id of the union
// joining them. Both profiles register their own cleanup hooks.
func createCoupleAndUnion(ctx context.Context, client *geni.Client) (*geni.ProfileResponse, *geni.ProfileResponse, string) {
	GinkgoHelper()

	focus := createFixtureProfile(ctx, client, "UnionFocus")

	partner, err := client.AddPartner(ctx, focus.Id)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		_ = client.DeleteProfile(context.Background(), partner.Id)
	})

	// AddPartner returns the new partner profile with the freshly
	// created union in its Unions list. Fall back to re-fetching the
	// focus if it's missing for any reason.
	var unionId string
	if len(partner.Unions) > 0 {
		unionId = partner.Unions[0]
	} else {
		got, err := client.GetProfile(ctx, focus.Id)
		Expect(err).ToNot(HaveOccurred())
		Expect(got.Unions).ToNot(BeEmpty())
		unionId = got.Unions[0]
	}
	return focus, partner, unionId
}

var _ = Describe("Union API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetUnion", func() {
		It("returns the union joining two newly-paired profiles", func() {
			focus, partner, unionId := createCoupleAndUnion(ctx, client)

			union, err := client.GetUnion(ctx, unionId)

			Expect(err).ToNot(HaveOccurred())
			Expect(union.Id).To(Equal(unionId))
			Expect(union.Partners).To(ConsistOf(focus.Id, partner.Id))
		})
	})

	Describe("GetUnions (bulk)", func() {
		// Single-id bulk requests are routed by the client through
		// a singular GetUnion call because Geni's server-side bulk
		// dispatcher returns empty for one-element ids lists. The
		// caller sees a normal *UnionBulkResponse; the workaround
		// is transparent. See union.go for the fallback.
		It("returns the requested union via the single-id fallback", func() {
			_, _, unionId := createCoupleAndUnion(ctx, client)

			res, err := client.GetUnions(ctx, []string{unionId})

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
			Expect(res.Results[0].Id).To(Equal(unionId))
		})
	})

	Describe("UpdateUnion", func() {
		It("sets a marriage year on the union", func() {
			_, _, unionId := createCoupleAndUnion(ctx, client)
			year := int32(1925)

			updated, err := client.UpdateUnion(ctx, unionId, &geni.UnionRequest{
				Marriage: &geni.EventElement{
					Date: &geni.DateElement{Year: &year},
				},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Id).To(Equal(unionId))
			Expect(updated.Marriage).ToNot(BeNil())
			Expect(updated.Marriage.Date).ToNot(BeNil())
			Expect(updated.Marriage.Date.Year).ToNot(BeNil())
			Expect(*updated.Marriage.Date.Year).To(BeEquivalentTo(1925))
		})
	})

	Describe("AddPartnerToUnion", func() {
		// Geni rejects a 3rd partner on a marriage with "Marriage
		// already has two partners", so the spec starts with a
		// single-parent union (created by adding a child to a fresh
		// profile) and adds a co-parent to it. Geni's docs claim this
		// endpoint returns a union; the live API returns the new
		// partner profile.
		It("creates a new partner profile bound to the union", func() {
			focus := createFixtureProfile(ctx, client, "PartnerToUnion")

			// AddChild on a sole profile creates a single-parent
			// union (focus as the only partner, new child as the
			// only child).
			child, err := client.AddChild(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			got, err := client.GetProfile(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Unions).ToNot(BeEmpty())
			unionId := got.Unions[0]

			partner, err := client.AddPartnerToUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())
			Expect(partner.Id).To(HavePrefix("profile-"))
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), partner.Id) })

			after, err := client.GetUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.Partners).To(ContainElement(partner.Id))
		})
	})

	Describe("AddChildToUnion", func() {
		It("creates a new child profile bound to the union", func() {
			_, _, unionId := createCoupleAndUnion(ctx, client)

			before, err := client.GetUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())

			child, err := client.AddChildToUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())
			Expect(child.Id).To(HavePrefix("profile-"))
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			after, err := client.GetUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.Children).To(ContainElement(child.Id))
			Expect(len(after.Children)).To(Equal(len(before.Children) + 1))
		})

		It("records `adopt` on the union's adopted_children list", func() {
			_, _, unionId := createCoupleAndUnion(ctx, client)

			child, err := client.AddChildToUnion(ctx, unionId, geni.WithModifier("adopt"))
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })

			after, err := client.GetUnion(ctx, unionId)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.AdoptedChildren).To(ContainElement(child.Id))
		})
	})
})
