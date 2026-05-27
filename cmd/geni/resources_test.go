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

func TestValidateResourceID(t *testing.T) {
	t.Run("valid prefix+digits passes", func(t *testing.T) {
		RegisterTestingT(t)
		for _, c := range []struct{ prefix, id string }{
			{"revision-", "revision-88812132160"},
			{"profile-", "profile-1"},
			{"document-", "document-44598467"},
		} {
			Expect(validateResourceID(c.prefix, c.id)).To(Succeed(), "valid %q", c.id)
		}
	})

	t.Run("bare numeric is rejected with actionable message", func(t *testing.T) {
		RegisterTestingT(t)
		err := validateResourceID("revision-", "88812132160")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("88812132160"))
		Expect(err.Error()).To(ContainSubstring("revision-"))
		Expect(err.Error()).To(ContainSubstring("revision-88812132160"))
	})

	t.Run("other bad shapes are rejected", func(t *testing.T) {
		RegisterTestingT(t)
		for _, bad := range []string{
			"",
			"profile-",
			"profile-abc",
			"revision-12345-extra",
			"profile-12345 ",
		} {
			Expect(validateResourceID("profile-", bad)).To(HaveOccurred(), "bad %q", bad)
		}
	})

	t.Run("wrong prefix is rejected", func(t *testing.T) {
		RegisterTestingT(t)
		err := validateResourceID("revision-", "profile-12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("revision-"))
	})
}

func TestResourceGet_RejectsBadIDBeforeAPICall(t *testing.T) {
	RegisterTestingT(t)
	h := resourceGet("revision-", func(_ *geni.Client, _ context.Context, _ string) (any, error) {
		panic("API must not be called when id validation rejects")
	})

	g := &globalOpts{stderr: io.Discard}
	err := h(context.Background(), g, []string{"88812132160"})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("revision-88812132160"))
}

func TestResourceGetBulk_RejectsAnyBadIDBeforeAPICall(t *testing.T) {
	RegisterTestingT(t)
	h := resourceGetBulk("revision-", func(_ *geni.Client, _ context.Context, _ []string) (any, error) {
		panic("API must not be called when any id is invalid")
	})

	g := &globalOpts{stderr: io.Discard}
	err := h(context.Background(), g, []string{"revision-1", "88812132160", "revision-2"})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("88812132160"))
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

func TestPrefixRevisionIDs(t *testing.T) {
	RegisterTestingT(t)
	Expect(prefixRevisionIDs([]string{"88793956740", "88793956730"})).
		To(Equal([]string{"revision-88793956740", "revision-88793956730"}))
}

func TestPrefixRevisionIDs_EmptyInputReturnsEmpty(t *testing.T) {
	RegisterTestingT(t)
	Expect(prefixRevisionIDs(nil)).To(BeEmpty())
	Expect(prefixRevisionIDs([]string{})).To(BeEmpty())
}

func TestRunDocumentTextGet_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentTextGet(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("two args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentTextGet(context.Background(), g, []string{"document-1", "document-2"})).To(HaveOccurred())
	})
}

func TestRunDocumentTextSet_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentTextSet(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("two positional args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentTextSet(context.Background(), g, []string{"document-1", "document-2"})).To(HaveOccurred())
	})

	t.Run("-from-file path that does not exist is an error", func(t *testing.T) {
		RegisterTestingT(t)
		err := runDocumentTextSet(context.Background(), g, []string{"-from-file", "/does/not/exist", "document-1"})
		Expect(err).To(HaveOccurred())
	})
}

func TestRunDocumentTextSet_RejectsEmptyStdin(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_CONSENT", "accepted")
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}

	err := runDocumentTextSet(context.Background(), g, []string{"document-1"})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("empty"))
}

func TestNormalizeDocumentText(t *testing.T) {
	t.Run("strips carriage returns", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(normalizeDocumentText("line1\r\nline2\r\n")).To(Equal("line1\nline2\n"))
	})

	t.Run("trims trailing whitespace per line", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(normalizeDocumentText("line1   \nline2\t\nline3 ")).To(Equal("line1\nline2\nline3"))
	})

	t.Run("CRLF + trailing spaces combined", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(normalizeDocumentText("a  \r\nb\r\n")).To(Equal("a\nb\n"))
	})

	t.Run("empty input → empty output", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(normalizeDocumentText("")).To(BeEmpty())
	})

	t.Run("idempotent", func(t *testing.T) {
		RegisterTestingT(t)
		once := normalizeDocumentText("a\r\nb \r\n")
		Expect(normalizeDocumentText(once)).To(Equal(once))
	})
}

func TestRunRevisionForProfile_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted") // skip the consent prompt

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runRevisionForProfile(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("two args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runRevisionForProfile(context.Background(), g, []string{"profile-1", "profile-2"})).To(HaveOccurred())
	})
}

func TestRunRevisionForProfile_AbortsWhenConsentDeclined(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GENI_WEB_CONSENT", "")

	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}

	err := runRevisionForProfile(context.Background(), g, []string{"6000000218702371879"})
	Expect(err).To(MatchError(ContainSubstring("declined")))
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
	handler := resourceGetBulk("profile-", func(*geni.Client, context.Context, []string) (any, error) {
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

func TestRunDocumentForProfile_ArgValidation(t *testing.T) {
	g := &globalOpts{stderr: io.Discard}

	t.Run("missing positional arg is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentForProfile(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("more than one positional arg is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentForProfile(context.Background(), g, []string{"profile-1", "profile-2"})).To(HaveOccurred())
	})

	t.Run("unknown flag is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runDocumentForProfile(context.Background(), g, []string{"-unknown", "profile-1"})).To(HaveOccurred())
	})
}
