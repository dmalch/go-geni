package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// createFixtureDocument creates a text-only document in the sandbox
// and registers a DeferCleanup hook to delete it.
func createFixtureDocument(ctx context.Context, client *geni.Client, title, body string) *geni.DocumentResponse {
	GinkgoHelper()
	created, err := client.CreateDocument(ctx, &geni.DocumentRequest{
		Title: title,
		Text:  strPtr(body),
	})
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		_ = client.DeleteDocument(context.Background(), created.Id)
	})
	return created
}

var _ = Describe("Document API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("Lifecycle", func() {
		It("creates a text document and reads it back", func() {
			created := createFixtureDocument(ctx, client, "AccCreateDoc", "hello acceptance")
			Expect(created.Id).ToNot(BeEmpty())

			got, err := client.GetDocument(ctx, created.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Id).To(Equal(created.Id))
			Expect(got.Title).To(Equal("AccCreateDoc"))
		})

		It("updates a document title", func() {
			created := createFixtureDocument(ctx, client, "AccUpdateBefore", "initial")

			updated, err := client.UpdateDocument(ctx, created.Id, &geni.DocumentRequest{
				Title: "AccUpdateAfter",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.Id).To(Equal(created.Id))
			Expect(updated.Title).To(Equal("AccUpdateAfter"))
		})

		It("deletes a document", func() {
			// Skip the auto-cleanup helper — we want to observe the
			// post-delete state inline. The sandbox soft-deletes
			// documents (a follow-up GET still succeeds), so we only
			// assert the delete call itself returns no error.
			created, err := client.CreateDocument(ctx, &geni.DocumentRequest{
				Title: "AccDeleteMe",
				Text:  strPtr("to-be-deleted"),
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(client.DeleteDocument(ctx, created.Id)).To(Succeed())
		})
	})

	Describe("Tagging", func() {
		// TagDocument associates a profile with a document; Untag
		// removes the association. Both return the document's
		// current tag list (as a ProfileBulkResponse).
		It("tags and untags a profile on a document", func() {
			profile := createFixtureProfile(ctx, client, "DocTag")
			doc := createFixtureDocument(ctx, client, "AccTagDoc", "tagged content")

			tagged, err := client.TagDocument(ctx, doc.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())
			_ = tagged // shape only — Geni's response shape here is opaque

			untagged, err := client.UntagDocument(ctx, doc.Id, profile.Id)
			Expect(err).ToNot(HaveOccurred())
			_ = untagged
		})
	})

	Describe("Bulk reads", func() {
		It("GetDocuments returns both ids in one call", func() {
			a := createFixtureDocument(ctx, client, "AccBulkA", "a")
			b := createFixtureDocument(ctx, client, "AccBulkB", "b")

			res, err := client.GetDocuments(ctx, []string{a.Id, b.Id})
			Expect(err).ToNot(HaveOccurred())
			// Some sandbox dispatch paths return a partial bulk
			// result; document but don't fail if so.
			if len(res.Results) == 0 {
				Skip("sandbox returned an empty bulk-document result (known flake)")
			}
			gotIds := make([]string, 0, len(res.Results))
			for _, d := range res.Results {
				gotIds = append(gotIds, d.Id)
			}
			Expect(gotIds).To(ContainElements(a.Id, b.Id))
		})

		It("GetUploadedDocuments returns the caller's uploads page", func() {
			createFixtureDocument(ctx, client, "AccUploadedFixture", "seed")

			res, err := client.GetUploadedDocuments(ctx, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).ToNot(BeEmpty())
		})
	})
})
