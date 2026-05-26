package acceptance

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/document"
)

// createFixtureDocument creates a text-only document in the sandbox
// and registers a DeferCleanup hook to delete it.
func createFixtureDocument(ctx context.Context, client *geni.Client, title, body string) *document.Document {
	GinkgoHelper()
	created, err := client.Document().Create(ctx, &document.Request{
		Title: title,
		Text:  new(body),
	})
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		_ = client.Document().Delete(context.Background(), created.ID)
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
			Expect(created.ID).ToNot(BeEmpty())

			got, err := client.Document().Get(ctx, created.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.ID).To(Equal(created.ID))
			Expect(got.Title).To(Equal("AccCreateDoc"))
		})

		It("updates a document title", func() {
			created := createFixtureDocument(ctx, client, "AccUpdateBefore", "initial")

			updated, err := client.Document().Update(ctx, created.ID, &document.Request{
				Title: "AccUpdateAfter",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.ID).To(Equal(created.ID))
			Expect(updated.Title).To(Equal("AccUpdateAfter"))
		})

		It("deletes a document and Get reports ErrResourceNotFound", func() {
			// Skip the auto-cleanup helper — we observe the post-delete
			// state inline. Geni returns HTTP 200 with the empty
			// bulk-envelope shape ({"results": []}) when a singular
			// /api/<id> GET targets a deleted document; the transport
			// coalescer translates that to ErrResourceNotFound so
			// callers can use the same is-deleted check as for any
			// missing resource.
			created, err := client.Document().Create(ctx, &document.Request{
				Title: "AccDeleteMe",
				Text:  new("to-be-deleted"),
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(client.Document().Delete(ctx, created.ID)).To(Succeed())

			_, err = client.Document().Get(ctx, created.ID)
			Expect(errors.Is(err, geni.ErrResourceNotFound)).To(BeTrue(),
				"expected ErrResourceNotFound after Delete, got %v", err)
		})
	})

	Describe("Tagging", func() {
		// TagDocument associates a profile with a document; Untag
		// removes the association. GetDocumentTags is the
		// authoritative read of the document's tagged-profiles
		// list — mirrors the GetPhotoTags pattern.
		It("tags, lists, and untags a profile on a document", func() {
			profile := createFixtureProfile(ctx, client, "DocTag")
			doc := createFixtureDocument(ctx, client, "AccTagDoc", "tagged content")

			_, err := client.Document().Tag(ctx, doc.ID, profile.ID)
			Expect(err).ToNot(HaveOccurred())

			tags, err := client.Document().Tags(ctx, doc.ID, 0)
			Expect(err).ToNot(HaveOccurred())
			ids := make([]string, 0, len(tags.Results))
			for _, p := range tags.Results {
				ids = append(ids, p.ID)
			}
			Expect(ids).To(ContainElement(profile.ID))

			_, err = client.Document().Untag(ctx, doc.ID, profile.ID)
			Expect(err).ToNot(HaveOccurred())

			tagsAfter, err := client.Document().Tags(ctx, doc.ID, 0)
			Expect(err).ToNot(HaveOccurred())
			idsAfter := make([]string, 0, len(tagsAfter.Results))
			for _, p := range tagsAfter.Results {
				idsAfter = append(idsAfter, p.ID)
			}
			Expect(idsAfter).ToNot(ContainElement(profile.ID))
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
				res, err := client.Document().GetBulk(ctx, []string{a.ID, b.ID})
				g.Expect(err).ToNot(HaveOccurred())
				gotIds := make([]string, 0, len(res.Results))
				for _, d := range res.Results {
					gotIds = append(gotIds, d.ID)
				}
				g.Expect(gotIds).To(ContainElements(a.ID, b.ID))
			}).
				WithTimeout(30 * time.Second).
				WithPolling(2 * time.Second).
				Should(Succeed())
		})

		It("GetUploadedDocuments returns the caller's uploads page", func() {
			createFixtureDocument(ctx, client, "AccUploadedFixture", "seed")

			res, err := client.User().UploadedDocuments(ctx, 1)
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

			_, err := client.Document().AddComment(ctx, doc.ID, body, "title-1")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				listed, err := client.Document().Comments(ctx, doc.ID, 0)
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

			res, err := client.Document().Projects(ctx, doc.ID, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})
})
