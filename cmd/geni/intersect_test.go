package main

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestIntersectIDs(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{name: "both empty", a: nil, b: nil, want: nil},
		{name: "a empty", a: nil, b: []string{"union-1"}, want: nil},
		{name: "b empty", a: []string{"union-1"}, b: nil, want: nil},
		{name: "disjoint", a: []string{"union-1", "union-2"}, b: []string{"union-3"}, want: nil},
		{name: "one overlap", a: []string{"union-1", "union-2"}, b: []string{"union-2", "union-3"}, want: []string{"union-2"}},
		{
			name: "multi overlap preserves b order",
			a:    []string{"union-1", "union-2", "union-3"},
			b:    []string{"union-3", "union-1"},
			want: []string{"union-3", "union-1"},
		},
		{
			name: "duplicates collapsed",
			a:    []string{"union-1", "union-1", "union-2"},
			b:    []string{"union-2", "union-2", "union-1"},
			want: []string{"union-2", "union-1"},
		},
		{
			name: "identical sets",
			a:    []string{"union-1", "union-2"},
			b:    []string{"union-1", "union-2"},
			want: []string{"union-1", "union-2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(intersectIDs(tc.a, tc.b)).To(Equal(tc.want))
		})
	}
}
