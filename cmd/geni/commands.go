package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sort"

	geni "github.com/dmalch/go-geni"
)

// commandTree returns the CLI command tree. Top-level entries are
// either flat commands (run set) or resource groups (sub set). It is a
// function rather than a package variable to avoid an initialization
// cycle: runHelp -> printUsage -> the tree.
func commandTree() map[string]*command {
	return map[string]*command{
		"login":  {summary: "authenticate and cache an OAuth token", run: runLogin},
		"logout": {summary: "delete the cached OAuth token", run: runLogout},
		"whoami": {summary: "show the authenticated user", run: runWhoami},
		"stats":  {summary: "show platform-wide statistics", run: runStats},
		"help":   {summary: "show this usage text", run: runHelp},

		"profile": {summary: "profile resource", sub: map[string]*command{
			"get": {summary: "fetch a profile by id", run: resourceGet("profile-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Profile().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple profiles by id", run: resourceGetBulk("profile-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Profile().GetBulk(ctx, ids)
				})},
			"search":  {summary: "search profiles by name", run: runProfileSearch},
			"open":    {summary: "open a profile's web page in the browser", run: runProfileOpen},
			"merge":   {summary: "merge one profile into another (destructive)", run: runProfileMerge},
			"compare": {summary: "compare two profiles field by field", run: runProfileCompare},
		}},
		"union": {summary: "union resource", sub: map[string]*command{
			"get": {summary: "fetch a union by id", run: resourceGet("union-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Union().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple unions by id", run: resourceGetBulk("union-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Union().GetBulk(ctx, ids)
				})},
			"intersect": {summary: "find unions two profiles share", run: runUnionIntersect},
		}},
		"document": {summary: "document resource", sub: map[string]*command{
			"for-profile": {summary: "list documents attached to a profile", run: runDocumentForProfile},
			"get": {summary: "fetch a document by id", run: resourceGet("document-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Document().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple documents by id", run: resourceGetBulk("document-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Document().GetBulk(ctx, ids)
				})},
			"open": {summary: "open a document's web page in the browser", run: runDocumentOpen},
			"text": {summary: "document text body (AJAX, one-time consent)", sub: map[string]*command{
				"get": {summary: "print a document's text body (raw, not JSON)", run: runDocumentTextGet},
				"set": {summary: "replace a document's text body (skips POST if unchanged)", run: runDocumentTextSet},
			}},
		}},
		"photo": {summary: "photo resource", sub: map[string]*command{
			"get": {summary: "fetch a photo by id", run: resourceGet("photo-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Photo().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple photos by id", run: resourceGetBulk("photo-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Photo().GetBulk(ctx, ids)
				})},
		}},
		"video": {summary: "video resource", sub: map[string]*command{
			"get": {summary: "fetch a video by id", run: resourceGet("video-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Video().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple videos by id", run: resourceGetBulk("video-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Video().GetBulk(ctx, ids)
				})},
		}},
		"photoalbum": {summary: "photo album resource", sub: map[string]*command{
			"get": {summary: "fetch a photo album by id", run: resourceGet("album-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.PhotoAlbum().Get(ctx, id)
				})},
		}},
		"project": {summary: "project resource", sub: map[string]*command{
			"get": {summary: "fetch a project by id", run: resourceGet("project-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Project().Get(ctx, id)
				})},
		}},
		"surname": {summary: "surname resource", sub: map[string]*command{
			"get": {summary: "fetch a surname by id", run: resourceGet("surname-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Surname().Get(ctx, id)
				})},
		}},
		"revision": {summary: "revision resource", sub: map[string]*command{
			"for-profile": {summary: "list a profile's revision IDs (AJAX, one-time consent)", run: runRevisionForProfile},
			"get": {summary: "fetch a revision by id", run: resourceGet("revision-",
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Revision().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple revisions by id", run: resourceGetBulk("revision-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Revision().GetBulk(ctx, ids)
				})},
		}},
		"config": {summary: "persisted CLI configuration (~/.genealogy/config.json)", sub: map[string]*command{
			"show":    {summary: "print the stored config as JSON", run: runConfigShow},
			"browser": {summary: "set or clear the default cookie-source browser", run: runConfigBrowser},
		}},
		"matches": {summary: "merge-center matches (AJAX, one-time consent)", sub: map[string]*command{
			"list":        {summary: "list profiles with pending tree/record/smart matches", run: runMatchesList},
			"for-profile": {summary: "tree-match candidates for one profile", run: runMatchesForProfile},
			"reject":      {summary: "reject a match candidate for a profile", run: runMatchesReject},
		}},
		"conflicts": {summary: "merge data-conflicts (AJAX, one-time consent)", sub: map[string]*command{
			"list":    {summary: "list profiles with unresolved data conflicts", run: runConflictsList},
			"show":    {summary: "show the conflicting fields for one profile", run: runConflictsShow},
			"resolve": {summary: "resolve a profile's data conflict (destructive)", run: runConflictsResolve},
		}},
		"tree": {summary: "family-graph queries", sub: map[string]*command{
			"family":    {summary: "immediate family of a profile", run: runTreeFamily},
			"ancestors": {summary: "ancestors of a profile", run: runTreeAncestors},
		}},
	}
}

// printUsage writes the command list to w.
func printUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, "geni — command-line client for the Geni.com API\n\n"+
		"Usage:\n  geni [-sandbox] <command> [<subcommand>] [flags] [args]\n\nCommands:\n")
	printCommands(w, "", commandTree())
	_, _ = fmt.Fprint(w, "\nGlobal flags:\n"+
		"  -sandbox    use sandbox.geni.com instead of production\n\n"+
		"Run \"geni login\" once to authenticate; the token is cached under ~/.genealogy.\n")
}

// printCommands recursively walks the command tree printing one line
// per leaf, with the full dotted path. Internal-only nodes (those with
// sub != nil) collapse into their leaves.
func printCommands(w io.Writer, prefix string, sub map[string]*command) {
	names := make([]string, 0, len(sub))
	for n := range sub {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := sub[n]
		path := n
		if prefix != "" {
			path = prefix + " " + n
		}
		if c.sub == nil {
			_, _ = fmt.Fprintf(w, "  %-30s %s\n", path, c.summary)
			continue
		}
		printCommands(w, path, c.sub)
	}
}

// runLogin performs the interactive OAuth handshake and caches the
// resulting token.
func runLogin(_ context.Context, g *globalOpts, _ []string) error {
	if os.Getenv("GENI_ACCESS_TOKEN") != "" {
		return errors.New("GENI_ACCESS_TOKEN is set; unset it to use cached browser auth")
	}
	ts, err := authChain(g.sandbox)
	if err != nil {
		return err
	}
	if _, err := ts.Token(); err != nil {
		return err
	}
	slog.Info("login successful", "sandbox", g.sandbox)
	return nil
}

// runLogout deletes the cached token file. It is a no-op when the file
// is already absent.
func runLogout(_ context.Context, g *globalOpts, _ []string) error {
	path, err := tokenCacheFilePath(g.sandbox)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	slog.Info("logged out", "cache", path)
	return nil
}

// runWhoami prints the account that owns the active token.
func runWhoami(ctx context.Context, g *globalOpts, _ []string) error {
	c, err := newClient(g)
	if err != nil {
		return err
	}
	u, err := c.User().Get(ctx)
	if err != nil {
		return err
	}
	return render(g.stdout, u)
}

// runStats prints Geni's platform-wide statistics.
func runStats(ctx context.Context, g *globalOpts, _ []string) error {
	c, err := newClient(g)
	if err != nil {
		return err
	}
	s, err := c.Stats().Get(ctx)
	if err != nil {
		return err
	}
	return render(g.stdout, s)
}

// runHelp prints the usage text to stdout.
func runHelp(_ context.Context, g *globalOpts, _ []string) error {
	printUsage(g.stdout)
	return nil
}
