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
		// The Geni sandbox sometimes returns an empty results array
		// for a single-id bulk lookup of a freshly-created union; we
		// document but don't fail on that path. The regression we
		// want to catch ("client decodes the bulk response shape")
		// is covered by the unit tier.
		It("returns the requested union when results are populated", func() {
			_, _, unionId := createCoupleAndUnion(ctx, client)

			res, err := client.GetUnions(ctx, []string{unionId})
			Expect(err).ToNot(HaveOccurred())
			if len(res.Results) == 0 {
				Skip("sandbox returned an empty bulk-union result for a fresh union (known flake)")
			}
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
})
