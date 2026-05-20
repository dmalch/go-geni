package main

import (
	"context"
	"errors"
	"flag"
	"strings"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/tree"
)

// resourceGet builds a leaf handler for a "get <id>" command: it reads
// exactly one id argument, constructs a client, calls get, and renders
// the result.
func resourceGet(get func(c *geni.Client, ctx context.Context, id string) (any, error)) func(context.Context, *globalOpts, []string) error {
	return func(ctx context.Context, g *globalOpts, args []string) error {
		if len(args) != 1 {
			return errors.New("expected exactly one <id> argument")
		}
		c, err := newClient(g)
		if err != nil {
			return err
		}
		v, err := get(c, ctx, args[0])
		if err != nil {
			return err
		}
		return render(g.stdout, v)
	}
}

// runProfileSearch handles "geni profile search [-page N] <name...>".
func runProfileSearch(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni profile search", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	page := fs.Int("page", 1, "result page, 1-based")
	if err := fs.Parse(args); err != nil {
		return err
	}
	names := strings.Join(fs.Args(), " ")
	if names == "" {
		return errors.New("usage: geni profile search [-page N] <name...>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	resp, err := c.Search().Profiles(ctx, names, *page)
	if err != nil {
		return err
	}
	return render(g.stdout, resp)
}

// runTreeFamily handles "geni tree family <profile-id>".
func runTreeFamily(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni tree family <profile-id>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	resp, err := c.Tree().ImmediateFamily(ctx, args[0])
	if err != nil {
		return err
	}
	return render(g.stdout, resp)
}

// runTreeAncestors handles "geni tree ancestors [-generations N] <profile-id>".
func runTreeAncestors(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni tree ancestors", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	generations := fs.Int("generations", 0, "ancestor generations to fetch, 0 uses the API default")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni tree ancestors [-generations N] <profile-id>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	var opts []tree.Option
	if *generations > 0 {
		opts = append(opts, tree.WithGenerations(*generations))
	}
	resp, err := c.Tree().Ancestors(ctx, fs.Arg(0), opts...)
	if err != nil {
		return err
	}
	return render(g.stdout, resp)
}
