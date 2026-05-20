package acceptance

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/profile"
)

var _ = Describe("Profile API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("Lifecycle", func() {
		It("creates a profile and reads it back", func() {
			created := createFixtureProfile(ctx, client, "CreateGet")
			Expect(created.Id).ToNot(BeEmpty())
			Expect(created.Guid).ToNot(BeEmpty())

			got, err := client.Profile().Get(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(created.Id))
			Expect(got.FirstName).ToNot(BeNil())
			Expect(*got.FirstName).To(Equal("CreateGet"))
		})

		It("updates an existing profile", func() {
			created := createFixtureProfile(ctx, client, "UpdateBefore")
			about := "Updated bio for acceptance test"

			updated, err := client.Profile().Update(ctx, created.Id, &profile.Request{
				AboutMe: &about,
				IsAlive: false,
				Public:  true,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Id).To(Equal(created.Id))
			Expect(updated.AboutMe).ToNot(BeNil())
			Expect(*updated.AboutMe).To(Equal(about))
		})

		It("deletes a profile", func() {
			// Allocate without the auto-cleanup helper — we want to
			// observe the post-delete state inside the spec.
			created, err := client.Profile().Create(ctx, &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: strPtr("DeleteMe"), LastName: strPtr("Acceptance")},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(client.Profile().Delete(ctx, created.Id)).To(Succeed())

			got, err := client.Profile().Get(ctx, created.Id)
			if errors.Is(err, geni.ErrResourceNotFound) {
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Deleted).To(BeTrue())
		})
	})

	Describe("Family additions", func() {
		// AddPartner / AddChild / AddSibling all return the new
		// profile; each registers its own cleanup hook.
		var focus *profile.Profile

		BeforeEach(func() {
			focus = createFixtureProfile(ctx, client, "FamilyFocus")
		})

		It("AddPartner returns a partner profile", func() {
			partner, err := client.Profile().AddPartner(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(partner.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), partner.Id) })
		})

		It("AddChild returns a child profile", func() {
			child, err := client.Profile().AddChild(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(child.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), child.Id) })
		})

		It("AddSibling returns a sibling profile", func() {
			sibling, err := client.Profile().AddSibling(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(sibling.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), sibling.Id) })
		})
	})

	Describe("Bulk reads", func() {
		It("GetProfiles returns both ids in one call", func() {
			a := createFixtureProfile(ctx, client, "BulkA")
			b := createFixtureProfile(ctx, client, "BulkB")

			res, err := client.Profile().GetBulk(ctx, []string{a.Id, b.Id})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect([]string{res.Results[0].Id, res.Results[1].Id}).To(ConsistOf(a.Id, b.Id))
		})

		It("GetManagedProfiles returns the caller's managed page", func() {
			// Seed at least one entry so the listing is non-empty
			// regardless of sandbox state.
			createFixtureProfile(ctx, client, "ManagedFixture")

			res, err := client.User().ManagedProfiles(ctx, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).ToNot(BeEmpty())
		})
	})

	Describe("MergeProfiles", func() {
		It("succeeds for two newly-created profiles", func() {
			keep := createFixtureProfile(ctx, client, "MergeKeep")
			dup, err := client.Profile().Create(ctx, &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: strPtr("MergeDup"), LastName: strPtr("Acceptance")},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			// dup may be consumed by merge; cleanup is a no-op on a
			// merged-away profile.
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), dup.Id) })

			Expect(client.Profile().Merge(ctx, keep.Id, dup.Id)).To(Succeed())
		})
	})

	Describe("WipeEventDates", func() {
		// Clears only the date sub-object of a named event (issue #94
		// in the parent provider). The profile is re-fetched to verify
		// the wipe took effect on the server.
		It("clears the date sub-object of a named event", func() {
			year := int32(1900)
			created, err := client.Profile().Create(ctx, &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: strPtr("WipeDates"), LastName: strPtr("Acceptance")},
				},
				Birth:   &profile.EventElement{Date: &profile.DateElement{Year: &year}},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), created.Id) })

			Expect(client.Profile().WipeEventDates(ctx, created.Id, []string{"birth"})).To(Succeed())

			got, err := client.Profile().Get(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			if got.Birth != nil && got.Birth.Date != nil {
				Expect(got.Birth.Date.Year).To(BeNil())
			}
		})
	})
})
