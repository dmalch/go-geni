package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"time"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/auth"
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
			"get": {summary: "fetch a profile by id (or -guid <guid>)", run: resourceGet("profile-", true,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Profile().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple profiles by id", run: resourceGetBulk("profile-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Profile().GetBulk(ctx, ids)
				})},
			"search":       {summary: "search profiles by name", run: runProfileSearch},
			"open":         {summary: "open a profile's web page in the browser", run: runProfileOpen},
			"merge":        {summary: "merge one profile into another (destructive)", run: runProfileMerge},
			"compare":      {summary: "compare two profiles field by field", run: runProfileCompare},
			"detach-union": {summary: "detach a profile from one or more unions (AJAX, mutating)", run: runProfileDetachUnion},
			"unions":       {summary: "list a profile's unions with their web ids", run: runProfileUnions},
		}},
		"union": {summary: "union resource", sub: map[string]*command{
			"get": {summary: "fetch a union by id", run: resourceGet("union-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Union().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple unions by id", run: resourceGetBulk("union-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Union().GetBulk(ctx, ids)
				})},
			"intersect": {summary: "find unions two profiles share", run: runUnionIntersect},
		}},
		"relationship": {summary: "parent/child relationship edits (AJAX, one-time consent)", sub: map[string]*command{
			"set-parent-modifier": {summary: "set a child's parent relationship modifier bio/adopt/foster (AJAX, mutating)", run: runRelationshipSetParentModifier},
		}},
		"document": {summary: "document resource", sub: map[string]*command{
			"for-profile": {summary: "list documents attached to a profile", run: runDocumentForProfile},
			"get": {summary: "fetch a document by id", run: resourceGet("document-", false,
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
			"get": {summary: "fetch a photo by id", run: resourceGet("photo-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Photo().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple photos by id", run: resourceGetBulk("photo-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Photo().GetBulk(ctx, ids)
				})},
		}},
		"video": {summary: "video resource", sub: map[string]*command{
			"get": {summary: "fetch a video by id", run: resourceGet("video-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Video().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple videos by id", run: resourceGetBulk("video-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Video().GetBulk(ctx, ids)
				})},
		}},
		"photoalbum": {summary: "photo album resource", sub: map[string]*command{
			"get": {summary: "fetch a photo album by id", run: resourceGet("album-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.PhotoAlbum().Get(ctx, id)
				})},
		}},
		"project": {summary: "project resource", sub: map[string]*command{
			"get": {summary: "fetch a project by id", run: resourceGet("project-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Project().Get(ctx, id)
				})},
		}},
		"surname": {summary: "surname resource", sub: map[string]*command{
			"get": {summary: "fetch a surname by id", run: resourceGet("surname-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Surname().Get(ctx, id)
				})},
		}},
		"revision": {summary: "revision resource", sub: map[string]*command{
			"for-profile": {summary: "list a profile's revision IDs (AJAX, one-time consent)", run: runRevisionForProfile},
			"get": {summary: "fetch a revision by id", run: resourceGet("revision-", false,
				func(c *geni.Client, ctx context.Context, id string) (any, error) {
					return c.Revision().Get(ctx, id)
				})},
			"get-bulk": {summary: "fetch multiple revisions by id", run: resourceGetBulk("revision-",
				func(c *geni.Client, ctx context.Context, ids []string) (any, error) {
					return c.Revision().GetBulk(ctx, ids)
				})},
		}},
		"config": {summary: "persisted CLI configuration (~/.genealogy/config.json)", sub: map[string]*command{
			"show":          {summary: "print the stored config as JSON (secrets redacted)", run: runConfigShow},
			"browser":       {summary: "set or clear the default cookie-source browser", run: runConfigBrowser},
			"client-id":     {summary: "set or clear the OAuth client id of your own Geni application", run: runConfigClientID},
			"client-secret": {summary: "set or clear the OAuth client secret, enabling refreshable logins", run: runConfigClientSecret},
		}},
		"token": {summary: "the cached OAuth access token", sub: map[string]*command{
			"print":  {summary: "print the access token, refreshing it if needed", run: runToken},
			"status": {summary: "report on the cached token without revealing it", run: runTokenStatus},
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
		"tree-conflicts": {summary: "merge tree-conflicts (AJAX, one-time consent)", sub: map[string]*command{
			"list":    {summary: "list profiles with unresolved tree conflicts", run: runTreeConflictsList},
			"show":    {summary: "inspect one profile's tree conflict and suggest fixes", run: runTreeConflictsShow},
			"resolve": {summary: "resolve an empty-parent-slot tree conflict by detaching the focus (AJAX, mutating)", run: runTreeConflictsResolve},
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
//
//	geni login [-port N]
//
// -port must match the Callback URL registered with the Geni
// application; it exists for callers running their own registration, not
// as a free choice.
func runLogin(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni login", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	port := fs.Int("port", -1, "port of the OAuth callback listener; must match the registered Callback URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Validate before the token check below, so a typo is reported the
	// same way whether or not GENI_ACCESS_TOKEN happens to be set.
	if *port < -1 || *port > 65535 {
		return fmt.Errorf("invalid -port %d", *port)
	}

	if os.Getenv("GENI_ACCESS_TOKEN") != "" {
		return errors.New("GENI_ACCESS_TOKEN is set; unset it to use cached browser auth")
	}

	opts := []auth.Option{auth.WithContext(ctx)}
	if *port >= 0 {
		opts = append(opts, auth.WithPort(*port))
	}

	_, refresher, _, err := authParts(g.sandbox, opts...)
	if err != nil {
		return err
	}
	ts, err := authChain(g.sandbox, opts...)
	if err != nil {
		return err
	}
	if _, err := ts.Token(); err != nil {
		return err
	}

	slog.Info("login successful", "sandbox", g.sandbox, "refreshable", refresher != nil)
	if refresher == nil {
		_, _ = fmt.Fprintln(g.stderr,
			"This token expires in a day and cannot be renewed. Store your application's\n"+
				"client secret with \"geni config client-secret <secret>\" to log in once and\n"+
				"let the CLI refresh in the background.")
	}
	return nil
}

// runToken prints the active access token, refreshing it first if it can.
// It makes the cached credential usable from a shell:
//
//	export GENI_ACCESS_TOKEN=$(geni token)
func runToken(_ context.Context, g *globalOpts, _ []string) error {
	ts, err := authChain(g.sandbox)
	if err != nil {
		return err
	}
	token, err := ts.Token()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(g.stdout, token.AccessToken)
	return nil
}

// runTokenStatus reports on the cached token without revealing it.
func runTokenStatus(_ context.Context, g *globalOpts, _ []string) error {
	cachePath, refresher, _, err := authParts(g.sandbox)
	if err != nil {
		return err
	}

	status := map[string]any{
		"environment": map[bool]string{true: "sandbox", false: "production"}[g.sandbox],
		"cache_path":  cachePath,
		"refreshable": refresher != nil,
	}

	if t := os.Getenv("GENI_ACCESS_TOKEN"); t != "" {
		status["source"] = "GENI_ACCESS_TOKEN"
		return render(g.stdout, status)
	}
	status["source"] = "cache"

	body, err := os.ReadFile(cachePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		status["logged_in"] = false
		return render(g.stdout, status)
	}

	var cached struct {
		Expiry       time.Time `json:"expiry"`
		RefreshToken string    `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &cached); err != nil {
		return fmt.Errorf("parse %s: %w", cachePath, err)
	}

	status["logged_in"] = true
	status["expires_at"] = cached.Expiry
	status["expired"] = !cached.Expiry.IsZero() && time.Now().After(cached.Expiry)
	status["has_refresh_token"] = cached.RefreshToken != ""
	return render(g.stdout, status)
}

// runLogout deletes the cached token file. It is a no-op when the file
// is already absent.
//
// It deliberately does not call /platform/oauth/invalidate_token: Geni
// issues one access token per application and user, so revoking it would
// also sign out every other tool sharing this application — the
// terraform-provider-genealogy that reads the same cache, for one.
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
