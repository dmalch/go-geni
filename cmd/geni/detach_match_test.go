package main

import (
	"testing"

	. "github.com/onsi/gomega"

	webtreeconflicts "github.com/dmalch/go-geni/web/treeconflicts"
)

// The Гусев case that exposed both defects: after splitting a wrongly merged
// family, Ерофей and Настасья share TWO unions — the stale one where she is his
// child, and the new one where they are partners. Both hold exactly the same
// two people, so a flat member set cannot tell them apart. The tree view also
// reports a partner slot the OAuth API omits ("-123725231f") and returns the
// marriage union twice.
const (
	erofej   = "6000000218702580846"
	nastasja = "6000000218702276862"
	marriage = "6000000226911569852"
	parental = "6000000218702849948"
)

func gusevUnions() []webtreeconflicts.WebUnion {
	return []webtreeconflicts.WebUnion{
		{WebID: marriage, Partners: []string{nastasja, erofej}, Children: nil},
		{WebID: parental, Partners: []string{"-123725231f", erofej}, Children: []string{nastasja}},
		{WebID: marriage, Partners: []string{nastasja, erofej}, Children: nil},
	}
}

func TestPickWebUnions(t *testing.T) {
	t.Run("Tells the parental union from the marriage by its children", func(t *testing.T) {
		RegisterTestingT(t)
		// OAuth union-123725231: partners [Ерофей], children [Настасья].
		got := pickWebUnions(gusevUnions(), []string{erofej}, []string{nastasja})
		Expect(got).To(Equal([]string{parental}))
	})

	t.Run("Tells the marriage from the parental union", func(t *testing.T) {
		RegisterTestingT(t)
		got := pickWebUnions(gusevUnions(), []string{erofej, nastasja}, nil)
		Expect(got).To(Equal([]string{marriage}))
	})

	t.Run("Counts a repeated union once", func(t *testing.T) {
		// The marriage appears twice above; before the fix that read as two
		// candidates and the command refused an unambiguous resolution.
		RegisterTestingT(t)
		Expect(pickWebUnions(gusevUnions(), []string{erofej, nastasja}, nil)).To(HaveLen(1))
	})

	t.Run("Ignores a tree-view partner the API does not report", func(t *testing.T) {
		// The parental union carries the "-123725231f" placeholder; requiring
		// partner-set equality would match nothing at all.
		RegisterTestingT(t)
		Expect(pickWebUnions(gusevUnions(), []string{erofej}, []string{nastasja})).
			To(Equal([]string{parental}))
	})

	t.Run("Returns nothing when the children differ", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(pickWebUnions(gusevUnions(), []string{erofej}, []string{"6000000999999999999"})).
			To(BeEmpty())
	})

	t.Run("Returns every candidate when two unions are truly alike", func(t *testing.T) {
		// Same partners AND same children in two distinct unions: genuinely
		// ambiguous, so both come back and the caller refuses to guess.
		RegisterTestingT(t)
		us := []webtreeconflicts.WebUnion{
			{WebID: "6000000000000000001", Partners: []string{erofej}, Children: []string{nastasja}},
			{WebID: "6000000000000000002", Partners: []string{erofej}, Children: []string{nastasja}},
		}
		Expect(pickWebUnions(us, []string{erofej}, []string{nastasja})).To(HaveLen(2))
	})

	t.Run("Member order does not matter", func(t *testing.T) {
		RegisterTestingT(t)
		us := []webtreeconflicts.WebUnion{
			{WebID: parental, Partners: []string{erofej, "x"}, Children: []string{"b", "a"}},
		}
		Expect(pickWebUnions(us, []string{erofej}, []string{"a", "b"})).To(Equal([]string{parental}))
	})
}

func TestPartnersOverlap(t *testing.T) {
	t.Run("Empty on either side defers to the children", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(partnersOverlap(nil, []string{erofej})).To(BeTrue())
		Expect(partnersOverlap([]string{erofej}, nil)).To(BeTrue())
	})

	t.Run("Disjoint partners do not overlap", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(partnersOverlap([]string{erofej}, []string{nastasja})).To(BeFalse())
	})
}
