package main

import (
	"context"
	"io"
	"strings"
	"testing"

	geni "github.com/dmalch/go-geni"
	. "github.com/onsi/gomega"
)

func TestConfirmed(t *testing.T) {
	RegisterTestingT(t)

	for _, in := range []string{"y", "Y", "yes", "YES", " yes \n", "y\n"} {
		Expect(confirmed(strings.NewReader(in))).To(BeTrue(), "input %q should confirm", in)
	}
	for _, in := range []string{"", "n", "no", "nope", "\n", "yeah", "1"} {
		Expect(confirmed(strings.NewReader(in))).To(BeFalse(), "input %q should not confirm", in)
	}
}

func TestRunProfileMerge_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}

	t.Run("fewer than two ids is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runProfileMerge(context.Background(), g, []string{"profile-1"})).To(HaveOccurred())
	})

	t.Run("more than two ids is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runProfileMerge(context.Background(), g, []string{"profile-1", "profile-2", "profile-3"})).To(HaveOccurred())
	})
}

func TestRunProfileMerge_AbortsWithoutConfirmation(t *testing.T) {
	t.Run("a 'no' answer aborts before any API call", func(t *testing.T) {
		RegisterTestingT(t)
		g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}
		err := runProfileMerge(context.Background(), g, []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(ContainSubstring("aborted")))
	})

	t.Run("empty stdin aborts (fail-safe, no API call)", func(t *testing.T) {
		RegisterTestingT(t)
		g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
		err := runProfileMerge(context.Background(), g, []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(ContainSubstring("aborted")))
	})
}

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
		// The arg-validation tests below all reject before reaching the
		// bulk fetch, so this body should never execute.
		panic("unreachable: arg validation should have rejected the call")
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
