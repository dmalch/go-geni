package main

import (
	"context"
	"errors"

	"github.com/dmalch/go-geni/union"
)

// runUnionIntersect handles "geni union intersect <pid1> <pid2>" — it
// fetches both profiles, intersects their union id lists, bulk-fetches
// the shared unions, and prints them as a JSON object keyed by union
// id.
func runUnionIntersect(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: geni union intersect <profile-id-1> <profile-id-2>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	a, err := c.Profile().Get(ctx, args[0])
	if err != nil {
		return err
	}
	b, err := c.Profile().Get(ctx, args[1])
	if err != nil {
		return err
	}

	ids := intersectIDs(a.Unions, b.Unions)
	out := make(map[string]*union.Union, len(ids))
	if len(ids) == 0 {
		return render(g.stdout, out)
	}

	resp, err := c.Union().GetBulk(ctx, ids)
	if err != nil {
		return err
	}
	for i := range resp.Results {
		u := resp.Results[i]
		out[u.ID] = &u
	}
	return render(g.stdout, out)
}

// intersectIDs returns the ids present in both a and b, in the order
// they first appear in b. Duplicates within either side are collapsed.
// Returns nil when the intersection is empty.
func intersectIDs(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(a))
	for _, id := range a {
		set[id] = struct{}{}
	}
	var out []string
	seen := make(map[string]struct{}, len(a))
	for _, id := range b {
		if _, ok := set[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
