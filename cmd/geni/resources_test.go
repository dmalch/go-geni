package main

import (
	"context"
	"testing"

	geni "github.com/dmalch/go-geni"
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

func TestSplitIDs(t *testing.T) {
	RegisterTestingT(t)

	// Space-separated args.
	Expect(splitIDs([]string{"profile-1", "profile-2", "profile-3"})).
		To(Equal([]string{"profile-1", "profile-2", "profile-3"}))

	// A single comma-separated arg.
	Expect(splitIDs([]string{"profile-1,profile-2,profile-3"})).
		To(Equal([]string{"profile-1", "profile-2", "profile-3"}))

	// Mixed, with surrounding blanks and an empty segment.
	Expect(splitIDs([]string{"profile-1, profile-2", "", "profile-3,"})).
		To(Equal([]string{"profile-1", "profile-2", "profile-3"}))

	// No usable ids.
	Expect(splitIDs(nil)).To(BeEmpty())
	Expect(splitIDs([]string{"", " ", ","})).To(BeEmpty())
}

func TestRunGetBulk_ArgValidation(t *testing.T) {
	g := &globalOpts{}
	handler := resourceGetBulk(func(*geni.Client, context.Context, []string) (any, error) {
		return nil, nil
	})

	t.Run("no ids is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(handler(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("only blank ids is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(handler(context.Background(), g, []string{",", " "})).To(HaveOccurred())
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
