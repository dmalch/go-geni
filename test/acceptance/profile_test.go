package acceptance

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
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

			got, err := client.GetProfile(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(created.Id))
			Expect(got.FirstName).ToNot(BeNil())
			Expect(*got.FirstName).To(Equal("CreateGet"))
		})

		It("updates an existing profile", func() {
			// Geni's update endpoint rejects detail_strings: null with
			// a 500 (Ruby NoMethodError); sending an empty map keeps
			// the server-side dispatcher happy.
			created := createFixtureProfile(ctx, client, "UpdateBefore")
			about := "Updated bio for acceptance test"

			updated, err := client.UpdateProfile(ctx, created.Id, &geni.ProfileRequest{
				AboutMe:       &about,
				DetailStrings: map[string]geni.DetailsString{},
				IsAlive:       false,
				Public:        true,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Id).To(Equal(created.Id))
			Expect(updated.AboutMe).ToNot(BeNil())
			Expect(*updated.AboutMe).To(Equal(about))
		})

		It("deletes a profile", func() {
			// Allocate without the auto-cleanup helper — we want to
			// observe the post-delete state inside the spec.
			created, err := client.CreateProfile(ctx, &geni.ProfileRequest{
				Names: map[string]geni.NameElement{
					"en-US": {FirstName: strPtr("DeleteMe"), LastName: strPtr("Acceptance")},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(client.DeleteProfile(ctx, created.Id)).To(Succeed())

			got, err := client.GetProfile(ctx, created.Id)
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
		var focus *geni.ProfileResponse

		BeforeEach(func() {
			focus = createFixtureProfile(ctx, client, "FamilyFocus")
		})

		It("AddPartner returns a partner profile", func() {
			partner, err := client.AddPartner(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(partner.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), partner.Id) })
		})

		It("AddChild returns a child profile", func() {
			child, err := client.AddChild(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(child.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), child.Id) })
		})

		It("AddSibling returns a sibling profile", func() {
			sibling, err := client.AddSibling(ctx, focus.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(sibling.Id).ToNot(BeEmpty())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), sibling.Id) })
		})
	})

	Describe("Bulk reads", func() {
		It("GetProfiles returns both ids in one call", func() {
			a := createFixtureProfile(ctx, client, "BulkA")
			b := createFixtureProfile(ctx, client, "BulkB")

			res, err := client.GetProfiles(ctx, []string{a.Id, b.Id})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect([]string{res.Results[0].Id, res.Results[1].Id}).To(ConsistOf(a.Id, b.Id))
		})

		It("GetManagedProfiles returns the caller's managed page", func() {
			// Seed at least one entry so the listing is non-empty
			// regardless of sandbox state.
			createFixtureProfile(ctx, client, "ManagedFixture")

			res, err := client.GetManagedProfiles(ctx, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).ToNot(BeEmpty())
		})
	})

	Describe("MergeProfiles", func() {
		It("succeeds for two newly-created profiles", func() {
			keep := createFixtureProfile(ctx, client, "MergeKeep")
			dup, err := client.CreateProfile(ctx, &geni.ProfileRequest{
				Names: map[string]geni.NameElement{
					"en-US": {FirstName: strPtr("MergeDup"), LastName: strPtr("Acceptance")},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			// dup may be consumed by merge; cleanup is a no-op on a
			// merged-away profile.
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), dup.Id) })

			Expect(client.MergeProfiles(ctx, keep.Id, dup.Id)).To(Succeed())
		})
	})

	Describe("WipeEventDates", func() {
		// Clears only the date sub-object of a named event (issue #94
		// in the parent provider). The profile is re-fetched to verify
		// the wipe took effect on the server.
		It("clears the date sub-object of a named event", func() {
			year := int32(1900)
			created, err := client.CreateProfile(ctx, &geni.ProfileRequest{
				Names: map[string]geni.NameElement{
					"en-US": {FirstName: strPtr("WipeDates"), LastName: strPtr("Acceptance")},
				},
				Birth:   &geni.EventElement{Date: &geni.DateElement{Year: &year}},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.DeleteProfile(context.Background(), created.Id) })

			Expect(client.WipeEventDates(ctx, created.Id, []string{"birth"})).To(Succeed())

			got, err := client.GetProfile(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			if got.Birth != nil && got.Birth.Date != nil {
				Expect(got.Birth.Date.Year).To(BeNil())
			}
		})
	})
})
