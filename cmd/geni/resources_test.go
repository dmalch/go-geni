package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestProfileWebURL(t *testing.T) {
	RegisterTestingT(t)

	// A profile-<n> id uses the /profile-<n> permalink.
	Expect(profileWebURL(false, "profile-122248213")).To(Equal("https://www.geni.com/profile-122248213"))
	Expect(profileWebURL(true, "profile-1")).To(Equal("https://sandbox.geni.com/profile-1"))

	// A bare guid uses the /people/id/<guid> permalink.
	Expect(profileWebURL(false, "6000000012102785219")).To(Equal("https://www.geni.com/people/id/6000000012102785219"))
	Expect(profileWebURL(true, "6000000012102785219")).To(Equal("https://sandbox.geni.com/people/id/6000000012102785219"))
}

func TestRunProfileOpen_ArgValidation(t *testing.T) {
	g := &globalOpts{}

	t.Run("no id is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runProfileOpen(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("more than one id is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runProfileOpen(context.Background(), g, []string{"profile-1", "profile-2"})).To(HaveOccurred())
	})
}

func TestDocumentWebURL(t *testing.T) {
	RegisterTestingT(t)
	Expect(documentWebURL(false, "6000000221744227924")).To(Equal("https://www.geni.com/documents/view?doc_id=6000000221744227924"))
	Expect(documentWebURL(true, "6000000221744227924")).To(Equal("https://sandbox.geni.com/documents/view?doc_id=6000000221744227924"))
}

func TestRunDocumentOpen_ArgValidation(t *testing.T) {
	g := &globalOpts{}

	t.Run("no id is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentOpen(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("more than one id is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentOpen(context.Background(), g, []string{"document-1", "document-2"})).To(HaveOccurred())
	})
}
