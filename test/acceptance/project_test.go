package acceptance

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// Project specs require a pre-existing sandbox project id: the API has
// no "create project" endpoint, and Geni's sandbox doesn't ship a
// fixture project. Set GENI_E2E_PROJECT_ID to a project the test
// account can collaborate on.
var _ = Describe("Project API", func() {
	var (
		ctx       context.Context
		client    *geni.Client
		projectId string
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()

		projectId = os.Getenv("GENI_E2E_PROJECT_ID")
		if projectId == "" {
			Skip("set GENI_E2E_PROJECT_ID to a sandbox project id the test account can collaborate on (e.g. project-12345)")
		}
	})

	Describe("GetProject", func() {
		It("returns the configured project", func() {
			project, err := client.GetProject(ctx, projectId)
			Expect(err).ToNot(HaveOccurred())
			Expect(project.Id).To(Equal(projectId))
		})
	})

	Describe("AddProfileToProject", func() {
		It("adds a freshly-created profile to the project", func() {
			profile := createFixtureProfile(ctx, client, "ProjectMember")

			res, err := client.AddProfileToProject(ctx, profile.Id, projectId)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("AddDocumentToProject", func() {
		It("adds a freshly-created document to the project", func() {
			doc := createFixtureDocument(ctx, client, "AccProjectDoc", "project content")

			res, err := client.AddDocumentToProject(ctx, doc.Id, projectId)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetProjectProfiles", func() {
		It("returns the project's profile list", func() {
			res, err := client.GetProjectProfiles(ctx, projectId, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetProjectCollaborators", func() {
		It("returns the project's collaborator list", func() {
			res, err := client.GetProjectCollaborators(ctx, projectId, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})

	Describe("GetProjectFollowers", func() {
		It("returns the project's follower list", func() {
			res, err := client.GetProjectFollowers(ctx, projectId, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
		})
	})
})
