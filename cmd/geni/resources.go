package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/tree"
	"github.com/skratchdot/open-golang/open"
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

// splitIDs flattens the args of a get-bulk command into an id list.
// Each arg may itself be comma-separated, so ids can be passed either
// space-separated, comma-separated, or a mix; blanks are dropped.
func splitIDs(args []string) []string {
	var ids []string
	for _, a := range args {
		for part := range strings.SplitSeq(a, ",") {
			if part = strings.TrimSpace(part); part != "" {
				ids = append(ids, part)
			}
		}
	}
	return ids
}

// resourceGetBulk builds a leaf handler for a "get-bulk <id...>"
// command: it parses the id list, constructs a client, calls the
// resource's bulk endpoint, and renders the results envelope.
func resourceGetBulk(getBulk func(c *geni.Client, ctx context.Context, ids []string) (any, error)) func(context.Context, *globalOpts, []string) error {
	return func(ctx context.Context, g *globalOpts, args []string) error {
		ids := splitIDs(args)
		if len(ids) == 0 {
			return errors.New("expected one or more ids (space- or comma-separated)")
		}
		c, err := newClient(g)
		if err != nil {
			return err
		}
		v, err := getBulk(c, ctx, ids)
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

// profileWebURL builds the browser URL for a profile id or guid. A
// "profile-<n>" id uses Geni's /profile-<n> permalink; a bare guid
// uses /people/id/<guid>. Both redirect to the canonical profile page.
func profileWebURL(sandbox bool, idOrGuid string) string {
	base := geni.BaseURL(sandbox)
	if strings.HasPrefix(idOrGuid, "profile-") {
		return base + idOrGuid
	}
	return base + "people/id/" + idOrGuid
}

// runProfileOpen handles "geni profile open <id-or-guid>" — it opens
// the profile's Geni web page in the default browser. The URL is built
// from the argument, so no API call or login is needed.
func runProfileOpen(_ context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("expected exactly one profile id or guid argument")
	}
	url := profileWebURL(g.sandbox, args[0])
	_, _ = fmt.Fprintf(g.stderr, "opening %s\n", url)
	return open.Start(url)
}

// confirmed reads a line from r and reports whether it is an
// affirmative answer ("y" or "yes", case-insensitive). EOF or any
// other input counts as a "no", so the prompt fails safe.
func confirmed(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// runProfileMerge handles "geni profile merge [-yes] <keep-id> <dup-id>"
// — it merges dup-id into keep-id. The merge is destructive and not
// easily undone, so it requires an interactive y/N confirmation unless
// -yes is passed.
func runProfileMerge(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni profile merge", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: geni profile merge [-yes] <keep-id> <duplicate-id>")
	}
	keepID, dupID := fs.Arg(0), fs.Arg(1)

	if !*yes {
		_, _ = fmt.Fprintf(g.stderr,
			"Merge %s into %s? This is destructive and cannot be easily undone. [y/N]: ",
			dupID, keepID)
		if !confirmed(g.stdin) {
			return errors.New("merge aborted")
		}
	}

	c, err := newClient(g)
	if err != nil {
		return err
	}
	res, err := c.Profile().Merge(ctx, keepID, dupID)
	if err != nil {
		return err
	}
	return render(g.stdout, res)
}

// documentWebURL builds the browser URL for a document guid. Unlike
// profiles, a document has no id-based permalink — its web page is
// reached only via the /documents/view?doc_id=<guid> route.
func documentWebURL(sandbox bool, guid string) string {
	return geni.BaseURL(sandbox) + "documents/view?doc_id=" + guid
}

// runDocumentOpen handles "geni document open <id-or-guid>" — it opens
// the document's Geni web page in the default browser. A bare guid is
// used directly; a "document-<n>" id is first resolved to its guid via
// the API (the document web page is keyed by guid, not id).
func runDocumentOpen(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("expected exactly one document id or guid argument")
	}

	guid := args[0]
	if strings.HasPrefix(guid, "document-") {
		c, err := newClient(g)
		if err != nil {
			return err
		}
		doc, err := c.Document().Get(ctx, guid)
		if err != nil {
			return err
		}
		if doc.Guid == "" {
			return errors.New("document has no guid")
		}
		guid = doc.Guid
	}

	url := documentWebURL(g.sandbox, guid)
	_, _ = fmt.Fprintf(g.stderr, "opening %s\n", url)
	return open.Start(url)
}

// runDocumentForProfile handles "geni document for-profile [-page N] <profile-id>".
func runDocumentForProfile(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni document for-profile", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	page := fs.Int("page", 1, "result page, 1-based")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni document for-profile [-page N] <profile-id>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	resp, err := c.Document().ForProfile(ctx, fs.Arg(0), *page)
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
