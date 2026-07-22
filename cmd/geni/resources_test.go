package main

import (
	"context"
	"io"
	"strings"
	"testing"

	geni "github.com/dmalch/go-geni"
	webmatches "github.com/dmalch/go-geni/web/matches"
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
	h := resourceGet("revision-", false, func(_ *geni.Client, _ context.Context, _ string) (any, error) {
		panic("API must not be called when id validation rejects")
	})

	g := &globalOpts{stderr: io.Discard}
	err := h(context.Background(), g, []string{"88812132160"})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("revision-88812132160"))
}

func TestResolveGetID(t *testing.T) {
	g := &globalOpts{stderr: io.Discard}

	t.Run("without allowGuid: strict prefix-NNN, no -guid flag", func(t *testing.T) {
		RegisterTestingT(t)
		id, err := resolveGetID("profile-", false, g, []string{"profile-123"})
		Expect(err).NotTo(HaveOccurred())
		Expect(id).To(Equal("profile-123"))

		// -guid is not a recognised flag when allowGuid is false, so the
		// arg count check rejects it (flag.Parse stops at the bare id).
		_, err = resolveGetID("profile-", false, g, []string{"6000000206907528877"})
		Expect(err).To(HaveOccurred())
	})

	t.Run("with allowGuid but no -guid: still strict prefix-NNN", func(t *testing.T) {
		RegisterTestingT(t)
		id, err := resolveGetID("profile-", true, g, []string{"profile-123"})
		Expect(err).NotTo(HaveOccurred())
		Expect(id).To(Equal("profile-123"))

		_, err = resolveGetID("profile-", true, g, []string{"6000000206907528877"})
		Expect(err).To(HaveOccurred())
	})

	t.Run("-guid rewrites a bare guid to the profile-g<guid> form", func(t *testing.T) {
		RegisterTestingT(t)
		id, err := resolveGetID("profile-", true, g, []string{"-guid", "6000000206907528877"})
		Expect(err).NotTo(HaveOccurred())
		Expect(id).To(Equal("profile-g6000000206907528877"))
	})

	t.Run("-guid rejects a non-numeric / prefixed argument", func(t *testing.T) {
		RegisterTestingT(t)
		for _, bad := range []string{"profile-123", "profile-g6000", "abc", ""} {
			_, err := resolveGetID("profile-", true, g, []string{"-guid", bad})
			Expect(err).To(HaveOccurred(), "bad guid %q", bad)
		}
	})

	t.Run("-guid requires exactly one positional argument", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := resolveGetID("profile-", true, g, []string{"-guid"})
		Expect(err).To(HaveOccurred())
		_, err = resolveGetID("profile-", true, g, []string{"-guid", "1", "2"})
		Expect(err).To(HaveOccurred())
	})
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

func TestRunMatchesList_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("positional args rejected", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesList(context.Background(), g, []string{"profile-1"})
		Expect(err).To(HaveOccurred())
	})

	t.Run("unknown -filter rejected before network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesList(context.Background(), g, []string{"-filter", "bogus"})
		Expect(err).To(MatchError(ContainSubstring("filter")))
	})

	t.Run("unknown -order rejected before network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesList(context.Background(), g, []string{"-order", "bogus"})
		Expect(err).To(MatchError(ContainSubstring("order")))
	})

	t.Run("unknown -collection rejected before network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesList(context.Background(), g, []string{"-collection", "bogus"})
		Expect(err).To(MatchError(ContainSubstring("collection")))
	})

	t.Run("unknown -direction rejected before network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesList(context.Background(), g, []string{"-direction", "sideways"})
		Expect(err).To(MatchError(ContainSubstring("direction")))
	})
}

func TestMatchesCollectionsContainsDefault(t *testing.T) {
	RegisterTestingT(t)
	// The CLI's default for -collection is "managed" — verify it
	// resolves through the lookup map. If someone changes the default,
	// the lookup must still contain the new key.
	Expect(matchesCollections["managed"]).To(Equal(webmatches.CollectionManaged))
}

func TestRunMatchesForProfile_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("no arg is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runMatchesForProfile(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("two args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runMatchesForProfile(context.Background(), g, []string{"profile-1", "profile-2"})).To(HaveOccurred())
	})

	t.Run("unknown -group rejected pre-network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runMatchesForProfile(context.Background(), g, []string{"-group", "bogus", "6000000218702371879"})
		Expect(err).To(MatchError(ContainSubstring("group")))
	})
}

func TestRunMatchesForProfile_AbortsWhenConsentDeclined(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GENI_WEB_CONSENT", "")

	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}

	err := runMatchesForProfile(context.Background(), g, []string{"6000000218702371879"})
	Expect(err).To(MatchError(ContainSubstring("declined")))
}

func TestRunMatchesList_AbortsWhenConsentDeclined(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GENI_WEB_CONSENT", "")

	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}

	err := runMatchesList(context.Background(), g, nil)
	Expect(err).To(MatchError(ContainSubstring("declined")))
}

func TestRunMatchesReject_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runMatchesReject(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("one arg is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runMatchesReject(context.Background(), g, []string{"6000000206102028412"})).To(HaveOccurred())
	})

	t.Run("three args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runMatchesReject(context.Background(), g, []string{"a", "b", "c"})).To(HaveOccurred())
	})
}

func TestRunMatchesReject_AbortsWithoutConfirmation(t *testing.T) {
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("a 'no' answer aborts before any reject", func(t *testing.T) {
		RegisterTestingT(t)
		g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}
		err := runMatchesReject(context.Background(), g, []string{"6000000206102028412", "6000000225685453832"})
		Expect(err).To(MatchError(ContainSubstring("aborted")))
	})

	t.Run("empty stdin aborts (fail-safe)", func(t *testing.T) {
		RegisterTestingT(t)
		g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
		err := runMatchesReject(context.Background(), g, []string{"6000000206102028412", "6000000225685453832"})
		Expect(err).To(MatchError(ContainSubstring("aborted")))
	})
}

func TestRunMatchesReject_AbortsWhenConsentDeclined(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GENI_WEB_CONSENT", "")

	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}

	err := runMatchesReject(context.Background(), g, []string{"6000000206102028412", "6000000225685453832"})
	Expect(err).To(MatchError(ContainSubstring("declined")))
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

func TestRunRelationshipSetParentModifier_ArgValidation(t *testing.T) {
	g := &globalOpts{stdin: strings.NewReader(""), stderr: io.Discard}
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runRelationshipSetParentModifier(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("one arg is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runRelationshipSetParentModifier(context.Background(), g, []string{"6000000218702637822"})).To(HaveOccurred())
	})

	t.Run("invalid modifier is rejected before any network", func(t *testing.T) {
		RegisterTestingT(t)
		err := runRelationshipSetParentModifier(context.Background(), g,
			[]string{"6000000218702637822", "sibling"})
		Expect(err).To(MatchError(ContainSubstring("modifier")))
	})
}

func TestRunRelationshipSetParentModifier_AbortsWithoutConfirmation(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_CONSENT", "accepted")
	// A bare guid avoids the OAuth guid lookup; "n" declines the mutation.
	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}
	err := runRelationshipSetParentModifier(context.Background(), g,
		[]string{"6000000218702637822", "foster"})
	Expect(err).To(MatchError(ContainSubstring("aborted")))
}

func TestRunRelationshipSetParentModifier_AbortsWhenConsentDeclined(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GENI_WEB_CONSENT", "")
	g := &globalOpts{stdin: strings.NewReader("n\n"), stderr: io.Discard}
	err := runRelationshipSetParentModifier(context.Background(), g,
		[]string{"6000000218702637822", "foster"})
	Expect(err).To(MatchError(ContainSubstring("declined")))
}
