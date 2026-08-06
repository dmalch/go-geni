package main

import (
	"bytes"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	webtreeconflicts "github.com/dmalch/go-geni/web/treeconflicts"
)

// Every paginated list command aggregates pages into `var out []T` and renders
// it. A nil slice marshals as `null`, so a command whose queue is empty printed
// `null` instead of `[]` — with exit 0 and no stderr. Consumers cannot tell that
// apart from a failure without inspecting the exit code, and a JSON list command
// should always emit a list. Observed on `tree-conflicts list` once the Merge
// Center queue was finally cleared (the web page read «Конфликты древ (0)» at
// the same moment); the same pattern was present in `matches list`,
// `conflicts list` and `union intersect`.
func TestRenderList(t *testing.T) {
	t.Run("An empty result renders as [] and never null", func(t *testing.T) {
		RegisterTestingT(t)
		var nothing []webtreeconflicts.TreeConflict // nil, as the aggregation starts
		var buf bytes.Buffer
		Expect(renderList(&buf, nothing)).To(Succeed())
		Expect(strings.TrimSpace(buf.String())).To(Equal("[]"))
	})

	t.Run("An empty non-nil slice also renders as []", func(t *testing.T) {
		RegisterTestingT(t)
		var buf bytes.Buffer
		Expect(renderList(&buf, []string{})).To(Succeed())
		Expect(strings.TrimSpace(buf.String())).To(Equal("[]"))
	})

	t.Run("A populated list is unchanged", func(t *testing.T) {
		RegisterTestingT(t)
		var buf bytes.Buffer
		Expect(renderList(&buf, []string{"a", "b"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring(`"a"`))
		Expect(buf.String()).To(ContainSubstring(`"b"`))
	})
}

// The Merge Center tree endpoints (/family-tree/index, /flash/*) are GUID-only:
// normalizeFlashProfileID strips the "profile-"/"profile-g" prefix but cannot
// turn an API short id into a guid. So `geni tree-conflicts show
// profile-34848631209` requested /family-tree/index/34848631209 and failed with
// an opaque "family-tree/index: HTTP 404", even though every sibling command
// (`profile get`, `conflicts show`, `profile compare`, `profile merge`) accepts
// exactly that id. The short form must be resolved to a guid via the API first.
//
// Note the THIRD id space this does not cover: the Merge Center's own web id
// (`short_id`, e.g. 407703728). It is not an API resource id at all — a lookup
// correctly reports "resource not found" — so it must never be printed in a
// `profile-NNN` shape that invites being passed here.
func TestNeedsGuidLookup(t *testing.T) {
	t.Run("An API short id needs a guid lookup", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(needsGuidLookup("profile-34848631209")).To(BeTrue())
	})

	t.Run("A profile-g guid does not — normalize already handles it", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(needsGuidLookup("profile-g6000000221407636318")).To(BeFalse())
	})

	t.Run("A bare guid does not", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(needsGuidLookup("6000000221407636318")).To(BeFalse())
	})
}
