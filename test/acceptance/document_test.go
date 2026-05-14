package acceptance

import (
	"context"
	"time"

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
		// Eventually-polled: sandbox sometimes returns an empty
		// bulk-document result for fresh ids on the first call. The
		// retry catches the propagation lag without papering over a
		// hard regression — if the ids never appear, the spec fails
		// at the timeout.
		It("GetDocuments eventually returns both ids in one call", func() {
			a := createFixtureDocument(ctx, client, "AccBulkA", "a")
			b := createFixtureDocument(ctx, client, "AccBulkB", "b")

			Eventually(func(g Gomega) {
				res, err := client.GetDocuments(ctx, []string{a.Id, b.Id})
				g.Expect(err).ToNot(HaveOccurred())
				gotIds := make([]string, 0, len(res.Results))
				for _, d := range res.Results {
					gotIds = append(gotIds, d.Id)
				}
				g.Expect(gotIds).To(ContainElements(a.Id, b.Id))
			}).
				WithTimeout(30 * time.Second).
				WithPolling(2 * time.Second).
				Should(Succeed())
		})

		It("GetUploadedDocuments returns the caller's uploads page", func() {
			createFixtureDocument(ctx, client, "AccUploadedFixture", "seed")

			res, err := client.GetUploadedDocuments(ctx, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).ToNot(BeEmpty())
		})
	})

	Describe("Comments", func() {
		// Skipped: AddDocumentComment returns success immediately,
		// but the comment never surfaces on GetDocumentComments in
		// the sandbox — verified by polling for 30s. Behaviour
		// matches the profile-media listings: the documented
		// listing semantics don't include the freshly-posted item,
		// or there's a filter/scope we don't have. The intended
		// Eventually assertion is preserved below so unskipping is a
		// one-line change once the sandbox starts propagating (or
		// once we understand the missing filter).
		It("posts a comment and eventually lists it back", func() {
			Skip("AddDocumentComment doesn't propagate to GetDocumentComments in the sandbox (polled 30s; comment never appeared)")

			doc := createFixtureDocument(ctx, client, "AccCommentDoc", "to-be-commented")
			body := "first comment"

			_, err := client.AddDocumentComment(ctx, doc.Id, body, "title-1")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				listed, err := client.GetDocumentComments(ctx, doc.Id, 0)
				g.Expect(err).ToNot(HaveOccurred())
				texts := make([]string, 0, len(listed.Results))
				for _, c := range listed.Results {
					texts = append(texts, c.Comment)
				}
				g.Expect(texts).To(ContainElement(body))
			}).
				WithTimeout(30 * time.Second).
				WithPolling(2 * time.Second).
				Should(Succeed())
		})
	})

	Describe("Projects", func() {
		// A freshly-created document is not in any project until the
		// caller adds it. We assert the call shape (succeeds, returns
		// a ProjectBulkResponse) rather than that the result set is
		// non-empty.
		It("returns the document's project list (possibly empty)", func() {
			doc := createFixtureDocument(ctx, client, "AccProjectsDoc", "project-listing")

			res, err := client.GetDocumentProjects(ctx, doc.Id, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})
})
