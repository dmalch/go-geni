package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/tree"
	webconflicts "github.com/dmalch/go-geni/web/conflicts"
	webdocument "github.com/dmalch/go-geni/web/document"
	webmatches "github.com/dmalch/go-geni/web/matches"
	webrelationships "github.com/dmalch/go-geni/web/relationships"
	webrevision "github.com/dmalch/go-geni/web/revision"
	webtreeconflicts "github.com/dmalch/go-geni/web/treeconflicts"
	webunions "github.com/dmalch/go-geni/web/unions"
	"github.com/skratchdot/open-golang/open"
)

// resourceIDPattern matches a Geni resource id: alphabetic prefix
// ending in "-" followed by one or more digits.
var resourceIDPattern = regexp.MustCompile(`^[a-z_]+-\d+$`)

// bareGuidPattern matches a Geni guid: a bare run of digits, no prefix.
var bareGuidPattern = regexp.MustCompile(`^\d+$`)

// validateResourceID returns nil if id is shaped like the expected
// resource id (prefix followed by digits), or an actionable error that
// names both the bad input and the correct form. The example in the
// error message rebuilds the user's input under the expected prefix so
// it doubles as a copy-paste fix when they typed a bare numeric id.
func validateResourceID(prefix, id string) error {
	if !resourceIDPattern.MatchString(id) || !strings.HasPrefix(id, prefix) {
		example := prefix + "<numeric-id>"
		if digits := strings.TrimLeft(id, "0123456789"); digits == "" && id != "" {
			example = prefix + id
		}
		return fmt.Errorf("invalid resource id %q: expected %q-prefixed form, e.g. %s", id, prefix, example)
	}
	return nil
}

// resourceGet builds a leaf handler for a "get <id>" command: it reads
// exactly one id argument, validates it against prefix, constructs a
// client, calls get, and renders the result.
//
// When allowGuid is set, the handler also accepts a -guid flag that
// reinterprets the argument as a bare numeric guid; see resolveGetID.
func resourceGet(prefix string, allowGuid bool, get func(c *geni.Client, ctx context.Context, id string) (any, error)) func(context.Context, *globalOpts, []string) error {
	return func(ctx context.Context, g *globalOpts, args []string) error {
		id, err := resolveGetID(prefix, allowGuid, g, args)
		if err != nil {
			return err
		}
		c, err := newClient(g)
		if err != nil {
			return err
		}
		v, err := get(c, ctx, id)
		if err != nil {
			return err
		}
		return render(g.stdout, v)
	}
}

// resolveGetID validates the single <id> argument of a "get" command and
// returns the id to query. With allowGuid set, a -guid flag lets the
// caller pass a bare numeric guid instead of a "<prefix>NNN" id; the guid
// is rewritten to Geni's immutable "<prefix>g<guid>" form (e.g.
// profile-g6000000206907528877), the only API shape that resolves a guid
// to a single resource — the bulk ids= endpoint silently ignores guids,
// so guid lookups are single-get only. Without allowGuid the behaviour is
// the strict "<prefix>NNN" form, unchanged.
func resolveGetID(prefix string, allowGuid bool, g *globalOpts, args []string) (string, error) {
	if !allowGuid {
		if len(args) != 1 {
			return "", errors.New("expected exactly one <id> argument")
		}
		if err := validateResourceID(prefix, args[0]); err != nil {
			return "", err
		}
		return args[0], nil
	}

	fs := flag.NewFlagSet("geni get", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	guid := fs.Bool("guid", false, "treat the argument as a bare numeric guid instead of a "+prefix+"NNN id")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() != 1 {
		return "", errors.New("expected exactly one <id> argument")
	}
	id := fs.Arg(0)
	if *guid {
		if !bareGuidPattern.MatchString(id) {
			return "", fmt.Errorf("invalid guid %q: expected a bare numeric guid (digits only) when -guid is set", id)
		}
		return prefix + "g" + id, nil
	}
	if err := validateResourceID(prefix, id); err != nil {
		return "", err
	}
	return id, nil
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
// command: it parses the id list, validates each entry against prefix,
// constructs a client, calls the resource's bulk endpoint, and renders
// the results envelope.
func resourceGetBulk(prefix string, getBulk func(c *geni.Client, ctx context.Context, ids []string) (any, error)) func(context.Context, *globalOpts, []string) error {
	return func(ctx context.Context, g *globalOpts, args []string) error {
		ids := splitIDs(args)
		if len(ids) == 0 {
			return errors.New("expected one or more ids (space- or comma-separated)")
		}
		for i, id := range ids {
			if err := validateResourceID(prefix, id); err != nil {
				return fmt.Errorf("id at position %d: %w", i+1, err)
			}
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

// normalizeDocumentText strips carriage returns and per-line trailing
// whitespace so two text bodies that differ only in line endings or
// trailing padding compare equal. Matches the rule used by
// geni-tree-terraform's update_documents.py.
func normalizeDocumentText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", "")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	return strings.Join(lines, "\n")
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

// runRevisionForProfile handles "geni revision for-profile <id-or-guid>"
// — it lists the revision IDs of a profile via the Web AJAX client.
// Accepts either a profile-NNN id (resolved to a guid via the OAuth
// API) or a bare guid (passed straight to the web client).
//
// Gated by ensureWebConsent: first invocation prompts y/N and writes
// ~/.genealogy/web_consent.json; subsequent calls skip the prompt.
func runRevisionForProfile(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni revision for-profile <profile-id-or-guid>")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}

	guid := args[0]
	if strings.HasPrefix(guid, "profile-") {
		c, err := newClient(g)
		if err != nil {
			return err
		}
		p, err := c.Profile().Get(ctx, guid)
		if err != nil {
			return err
		}
		if p.Guid == "" {
			return errors.New("profile has no guid")
		}
		guid = p.Guid
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}

	ids, err := webrevision.NewClient(wc).ForProfile(ctx, guid)
	if err != nil {
		return err
	}
	return render(g.stdout, prefixRevisionIDs(ids))
}

// prefixRevisionIDs adapts the AJAX endpoint's bare numeric rev_ids
// into the "revision-NNN" form the OAuth API (and `geni revision get`)
// expects, so the output chains directly into `xargs … geni revision get`.
func prefixRevisionIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = "revision-" + id
	}
	return out
}

// resolveDocumentGuid returns the document's guid for either a
// "document-NNN" id (one OAuth Get call) or a bare guid (returned
// verbatim). Mirrors runDocumentOpen's resolution path so the AJAX
// text commands accept the same id shapes as the rest of the document
// CLI.
func resolveDocumentGuid(ctx context.Context, g *globalOpts, idOrGuid string) (string, error) {
	if !strings.HasPrefix(idOrGuid, "document-") {
		return idOrGuid, nil
	}
	c, err := newClient(g)
	if err != nil {
		return "", err
	}
	d, err := c.Document().Get(ctx, idOrGuid)
	if err != nil {
		return "", err
	}
	if d.Guid == "" {
		return "", errors.New("document has no guid")
	}
	return d.Guid, nil
}

// runDocumentTextGet handles "geni document text get <id-or-guid>" —
// it prints the document's text body on stdout. AJAX-backed; gated by
// the one-time web consent prompt. Stdout is the raw text (not JSON)
// since the artifact requested is text, not a record.
func runDocumentTextGet(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni document text get <document-id-or-guid>")
	}
	if err := ensureWebConsent(g); err != nil {
		return err
	}
	guid, err := resolveDocumentGuid(ctx, g, args[0])
	if err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	text, err := webdocument.NewClient(wc).GetText(ctx, guid)
	if err != nil {
		return err
	}
	_, err = io.WriteString(g.stdout, text)
	return err
}

// runDocumentTextSet handles
//
//	geni document text set [-from-file <path>] <document-id-or-guid>
//
// It reads the new body from -from-file or stdin, fetches the current
// body, and POSTs only when the normalized bodies differ. Output is
// JSON: {"status":"updated"|"unchanged","guid":"…","bytes_written":N}.
func runDocumentTextSet(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni document text set", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	fromFile := fs.String("from-file", "", "read new body from this file instead of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni document text set [-from-file <path>] <document-id-or-guid>")
	}
	if err := ensureWebConsent(g); err != nil {
		return err
	}

	var newBody []byte
	if *fromFile != "" {
		b, err := os.ReadFile(*fromFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", *fromFile, err)
		}
		newBody = b
	} else {
		b, err := io.ReadAll(g.stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		newBody = b
	}
	if len(newBody) == 0 {
		return errors.New("new body is empty (read from " +
			map[bool]string{true: "stdin", false: "-from-file"}[*fromFile == ""] +
			"); refusing to overwrite document text with nothing")
	}

	guid, err := resolveDocumentGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	dc := webdocument.NewClient(wc)

	current, err := dc.GetText(ctx, guid)
	if err != nil {
		return err
	}
	if normalizeDocumentText(current) == normalizeDocumentText(string(newBody)) {
		return render(g.stdout, map[string]any{"status": "unchanged", "guid": guid})
	}
	if err := dc.SaveText(ctx, guid, string(newBody)); err != nil {
		return err
	}
	return render(g.stdout, map[string]any{
		"status":        "updated",
		"guid":          guid,
		"bytes_written": len(newBody),
	})
}

// matchesCollections maps user-facing -collection values to the Geni
// query-string values. Both keys map identically since the underlying
// names are already user-readable.
var matchesCollections = map[string]webmatches.Collection{
	"":              "",
	"managed":       webmatches.CollectionManaged,
	"relatives":     webmatches.CollectionRelatives,
	"followed":      webmatches.CollectionFollowed,
	"collaborators": webmatches.CollectionCollaborators,
}

// matchesFilters maps the user-facing -filter values to the Geni
// query-string values. Shortened on the CLI side because the
// "_matches" suffix is redundant under `geni matches list`.
var matchesFilters = map[string]webmatches.Filter{
	"":       "",
	"tree":   webmatches.FilterTreeMatches,
	"record": webmatches.FilterRecordMatches,
	"smart":  webmatches.FilterSmartMatches,
}

// matchesOrders maps user-facing -order values to the Geni
// query-string values (some of which are unintuitive — "value_add"
// is the matches column, "mc_updated_at" is the updated_at column).
var matchesOrders = map[string]webmatches.Order{
	"":             "",
	"name":         webmatches.OrderName,
	"relationship": webmatches.OrderRelationship,
	"manager":      webmatches.OrderManager,
	"updated_at":   webmatches.OrderUpdatedAt,
	"matches":      webmatches.OrderMatches,
}

var matchesDirections = map[string]webmatches.Direction{
	"":     "",
	"asc":  webmatches.DirectionAsc,
	"desc": webmatches.DirectionDesc,
}

// matchesGroups maps the user-facing -group values to the
// /search/matches/<guid> query-string values. "new" is the default
// (omitted from URL).
var matchesGroups = map[string]webmatches.Group{
	"":          webmatches.GroupNew,
	"new":       webmatches.GroupNew,
	"requested": webmatches.GroupRequested,
	"removed":   webmatches.GroupRemoved,
}

// runMatchesList handles
//
//	geni matches list [-collection X] [-filter Y] [-order Z] [-direction D] \
//	                  [-page N | -all] [-limit N]
//
// It paginates the merge-center matches list via the Web AJAX client.
// Output is a JSON array of match entries. Gated by ensureWebConsent.
func runMatchesList(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni matches list", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	collection := fs.String("collection", "managed", "{managed,relatives,followed,collaborators}")
	filter := fs.String("filter", "", "{tree,record,smart}")
	order := fs.String("order", "", "{name,relationship,manager,updated_at,matches}")
	direction := fs.String("direction", "", "{asc,desc}")
	page := fs.Int("page", 0, "1-based page number; ignored with -all")
	all := fs.Bool("all", false, "paginate until no next page")
	limit := fs.Int("limit", 0, "cap output rows after pagination (0 = no cap)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: geni matches list [flags] (no positional args)")
	}

	col, ok := matchesCollections[*collection]
	if !ok {
		return fmt.Errorf("invalid -collection %q (want one of: managed, relatives, followed, collaborators)", *collection)
	}
	flt, ok := matchesFilters[*filter]
	if !ok {
		return fmt.Errorf("invalid -filter %q (want one of: tree, record, smart)", *filter)
	}
	ord, ok := matchesOrders[*order]
	if !ok {
		return fmt.Errorf("invalid -order %q (want one of: name, relationship, manager, updated_at, matches)", *order)
	}
	dir, ok := matchesDirections[*direction]
	if !ok {
		return fmt.Errorf("invalid -direction %q (want one of: asc, desc)", *direction)
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	mc := webmatches.NewClient(wc)

	startPage := *page
	if startPage <= 0 {
		startPage = 1
	}

	var out []webmatches.Match
	for p := startPage; ; p++ {
		res, err := mc.List(ctx, webmatches.ListOptions{
			Collection: col,
			Filter:     flt,
			Order:      ord,
			Direction:  dir,
			Page:       p,
		})
		if err != nil {
			return err
		}
		out = append(out, res.Matches...)
		if !*all || !res.HasNext {
			break
		}
		if *limit > 0 && len(out) >= *limit {
			break
		}
	}

	if *limit > 0 && len(out) > *limit {
		out = out[:*limit]
	}
	return render(g.stdout, out)
}

// runMatchesForProfile handles
//
//	geni matches for-profile [-group new|requested|removed] <profile-id-or-guid>
//
// It fetches /search/matches/<guid> and parses the source profile +
// candidate tree matches. Accepts either a profile-NNN id (resolved
// to a guid via the OAuth API) or a bare guid. Gated by
// ensureWebConsent.
func runMatchesForProfile(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni matches for-profile", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	group := fs.String("group", "new", "{new,requested,removed}")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni matches for-profile [-group X] <profile-id-or-guid>")
	}
	grp, ok := matchesGroups[*group]
	if !ok {
		return fmt.Errorf("invalid -group %q (want one of: new, requested, removed)", *group)
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}

	guid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	res, err := webmatches.NewClient(wc).ForProfile(ctx, guid, webmatches.ForProfileOptions{Group: grp})
	if err != nil {
		return err
	}
	return render(g.stdout, res)
}

// runMatchesReject handles
//
//	geni matches reject [-yes] <source-profile-id-or-guid> <match-profile-id-or-guid>
//
// It removes the pending match between the source profile and the match
// candidate (the "Удалить совпадение" action in the merge center).
// Either argument may be a profile-NNN id (resolved to a guid via the
// OAuth API) or a bare guid. The reject is reversible — rejected matches
// move to the "removed" group, viewable via `matches for-profile -group
// removed` — but a y/N confirmation is still required unless -yes is
// passed. Gated by ensureWebConsent.
func runMatchesReject(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni matches reject", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: geni matches reject [-yes] <source-profile-id-or-guid> <match-profile-id-or-guid>")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}

	sourceGuid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}
	matchGuid, err := resolveProfileGuid(ctx, g, fs.Arg(1))
	if err != nil {
		return err
	}

	if !*yes {
		_, _ = fmt.Fprintf(g.stderr,
			"Reject match %s for profile %s? [y/N]: ", matchGuid, sourceGuid)
		if !confirmed(g.stdin) {
			return errors.New("reject aborted")
		}
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	if err := webmatches.NewClient(wc).Reject(ctx, sourceGuid, matchGuid); err != nil {
		return err
	}
	return render(g.stdout, map[string]string{
		"status": "rejected",
		"source": sourceGuid,
		"match":  matchGuid,
	})
}

// runProfileDetachUnion handles
//
//	geni profile detach-union [-yes] <profile-id-or-guid> <union-id> [<union-id>…]
//
// It removes the profile's connection to one or more unions — the
// "Удалить связь" / "remove relationships" action on the profile's
// edit_relationships page — by POSTing to
// /profile_actions/delete_relationships. Detaching does not delete the
// union (an emptied union becomes a harmless orphan), and re-attaching
// requires the web UI, so a y/N confirmation is required unless -yes is
// passed. Like the other web commands it is gated by the one-time AJAX
// consent prompt.
//
// The union arguments may be either Geni web ids (the bare 6000000…
// numbers shown as remove_connection_<id> on the edit_relationships
// page) or the short OAuth "union-NNN" ids that `geni union get` and the
// Terraform provider use. The OAuth API exposes no union guid, so a
// short id is resolved by membership: the profile's unions are read off
// the tree view (which is the only source of the web id) and matched
// against the OAuth union's partners+children.
//
// Every id is validated against that list before anything is POSTed. An
// id the profile does not actually belong to is a hard error — Geni
// silently ignores an unknown union and would otherwise let this command
// report a detach that never happened.
func runProfileDetachUnion(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni profile detach-union", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return errors.New("usage: geni profile detach-union [-yes] <profile-id-or-guid> <union-id> [<union-id>…]")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}

	profileGuid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}
	unionIDs := make([]string, 0, fs.NArg()-1)
	for _, raw := range fs.Args()[1:] {
		uid, err := normalizeUnionID(raw)
		if err != nil {
			return err
		}
		unionIDs = append(unionIDs, uid)
	}
	unionIDs, err = resolveDetachUnionIDs(ctx, g, fs.Arg(0), profileGuid, unionIDs)
	if err != nil {
		return err
	}

	if !*yes {
		_, _ = fmt.Fprintf(g.stderr,
			"Detach profile %s from union(s) %s? [y/N]: ", profileGuid, strings.Join(unionIDs, ", "))
		if !confirmed(g.stdin) {
			return errors.New("detach aborted")
		}
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	if err := webunions.NewClient(wc).Detach(ctx, profileGuid, unionIDs); err != nil {
		return err
	}
	return render(g.stdout, map[string]any{
		"status":  "detached",
		"profile": profileGuid,
		"unions":  unionIDs,
	})
}

// runRelationshipSetParentModifier handles
//
//	geni relationship set-parent-modifier [-parent-union <id>] [-yes] <child-id-or-guid> <bio|adopt|foster>
//
// It changes the relationship modifier of the child's edge to one of its
// parent unions — the parent_modifiers[…] control on the child's
// edit_relationships page — by refetching the form, flipping the one
// field, and POSTing the whole form back the way the browser's Save does.
// The OAuth API can attach a child with a modifier but cannot re-tag an
// existing biological edge, so this is the only path to move a live
// parent↔child edge to foster/adopted. Mutating, so a y/N confirmation is
// required unless -yes; gated by the one-time AJAX consent prompt.
//
// -parent-union is a Geni web union id (the bare 6000000… number,
// optionally union- prefixed); omit it to target the child's sole parent
// union (an error when the child has more than one). A short OAuth
// union-NNN has no web guid and will not resolve.
func runRelationshipSetParentModifier(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni relationship set-parent-modifier", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	parentUnion := fs.String("parent-union", "", "web union id to re-tag (default: the child's sole parent union)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: geni relationship set-parent-modifier [-parent-union <id>] [-yes] " +
			"<child-id-or-guid> <bio|adopt|foster>")
	}
	modifier := fs.Arg(1)
	if !webrelationships.ValidModifier(modifier) {
		return fmt.Errorf("invalid modifier %q: want %s, %s or %s", modifier,
			webrelationships.ModifierBiological, webrelationships.ModifierAdopted, webrelationships.ModifierFoster)
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}

	childGuid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}
	var unionWebID string
	if *parentUnion != "" {
		unionWebID, err = normalizeUnionID(*parentUnion)
		if err != nil {
			return err
		}
	}

	if !*yes {
		via := ""
		if unionWebID != "" {
			via = " via union " + unionWebID
		}
		_, _ = fmt.Fprintf(g.stderr,
			"Set parent relationship of %s%s to %q? [y/N]: ", childGuid, via, modifier)
		if !confirmed(g.stdin) {
			return errors.New("set-parent-modifier aborted")
		}
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	res, err := webrelationships.NewClient(wc).SetParentModifier(ctx, childGuid, unionWebID, modifier)
	if err != nil {
		return err
	}
	status := "updated"
	if !res.Changed {
		status = "unchanged"
	}
	return render(g.stdout, map[string]any{
		"status":   status,
		"profile":  childGuid,
		"union":    res.Union,
		"modifier": res.Modifier,
	})
}

// normalizeUnionID accepts a union web id as a bare run of digits or in
// "union-<digits>" form and returns the bare-digit form used by the
// delete_relationships endpoint's uids parameter.
func normalizeUnionID(id string) (string, error) {
	bare := strings.TrimPrefix(id, "union-")
	if !bareGuidPattern.MatchString(bare) {
		return "", fmt.Errorf("invalid union id %q: expected a web union id (the digits in "+
			"remove_connection_<id> on the edit_relationships page), optionally union- prefixed", id)
	}
	return bare, nil
}

// runProfileUnions handles
//
//	geni profile unions <profile-id-or-guid>
//
// It prints every union the profile belongs to as the tree view reports it,
// each with the Geni WEB id. That id is what `profile detach-union` and the
// edit_relationships page speak, and the OAuth API never exposes it — so
// without this there is no way to go from a `union-NNN` (which `union get`
// and the Terraform provider use) to something detachable. Read-only.
func runProfileUnions(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni profile unions <profile-id-or-guid>")
	}
	if err := ensureWebConsent(g); err != nil {
		return err
	}
	profileGuid, err := resolveProfileGuid(ctx, g, args[0])
	if err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	us, err := webtreeconflicts.NewClient(wc).UnionsFor(ctx, profileGuid)
	if err != nil {
		return err
	}
	return render(g.stdout, map[string]any{"profile": profileGuid, "unions": us})
}

// resolveDetachUnionIDs turns each requested union id into the web id that
// /profile_actions/delete_relationships understands, and rejects any id the
// profile does not belong to.
//
// Two forms are accepted. A web id (6000000…) is passed through once it is
// confirmed to be one of the profile's unions. A short OAuth id (union-NNN,
// which is what `geni union get` and the Terraform provider speak) carries no
// web counterpart in the API, so it is matched by MEMBERSHIP: the OAuth union's
// partners+children must equal one tree-view union's. Containment is not enough
// — a person's two unions routinely share a partner, and picking the wrong one
// would detach the wrong family.
func resolveDetachUnionIDs(ctx context.Context, g *globalOpts, profileArg, profileGuid string,
	requested []string) ([]string, error) {
	cookies, err := loadWebCookies(g)
	if err != nil {
		return nil, err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return nil, err
	}
	webUnions, err := webtreeconflicts.NewClient(wc).UnionsFor(ctx, profileGuid)
	if err != nil {
		return nil, fmt.Errorf("looking up the profile's unions: %w", err)
	}
	if len(webUnions) == 0 {
		return nil, fmt.Errorf("profile %s belongs to no unions the tree view reports; nothing to detach", profileArg)
	}

	byWebID := make(map[string]webtreeconflicts.WebUnion, len(webUnions))
	known := make([]string, 0, len(webUnions))
	for _, wu := range webUnions {
		// Distinct ids only: the tree view repeats a union, and listing it
		// twice in an error reads like the profile has two of them.
		if _, dup := byWebID[wu.WebID]; dup {
			continue
		}
		byWebID[wu.WebID] = wu
		known = append(known, wu.WebID)
	}

	out := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, ok := byWebID[id]; ok {
			out = append(out, id)
			continue
		}
		webID, err := matchOAuthUnion(ctx, g, id, profileArg, webUnions, known)
		if err != nil {
			return nil, err
		}
		out = append(out, webID)
	}
	return out, nil
}

// matchOAuthUnion resolves a short OAuth union id to a web id by comparing the
// union with each of the profile's tree-view unions.
//
// Matching is on the CHILDREN set plus a shared partner, not on the flat
// membership. Two unions of the same pair routinely hold the same people in
// different roles — after splitting a wrongly-merged family, a man and a woman
// can be partners in one union and parent-and-child in another — and a flat
// member set cannot tell those apart. It also cannot be made to: the tree view
// reports partners the OAuth API does not (an unresolved slot shows up as a
// placeholder like "-123725231f"), so partner sets are compared for overlap
// rather than equality, while children compare exactly.
func matchOAuthUnion(ctx context.Context, g *globalOpts, id, profileArg string,
	webUnions []webtreeconflicts.WebUnion, known []string) (string, error) {
	c, err := newClient(g)
	if err != nil {
		return "", err
	}
	u, err := c.Union().Get(ctx, "union-"+id)
	if err != nil {
		return "", fmt.Errorf("union id %q is not one of profile %s's unions (%s) and does not "+
			"resolve as an OAuth union-%s: %w", id, profileArg, strings.Join(known, ", "), id, err)
	}

	// Compare GUIDS: the tree view's member ids live in its own space, so the
	// union's OAuth member ids have to be resolved to guids first.
	guidOf, err := memberGuids(ctx, c, append(append([]string{}, u.Partners...), u.Children...))
	if err != nil {
		return "", fmt.Errorf("resolving union-%s's members to guids: %w", id, err)
	}
	wantPartners := normalizeMembers(mapGuids(u.Partners, guidOf))
	wantChildren := normalizeMembers(mapGuids(u.Children, guidOf))

	matches := pickWebUnions(webUnions, wantPartners, wantChildren)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("union-%s does not match any union of profile %s by children and "+
			"partners (its unions are %s) — the tree view and the API disagree, so refusing to guess",
			id, profileArg, strings.Join(known, ", "))
	default:
		return "", fmt.Errorf("union-%s matches %d distinct unions of profile %s (%s) — "+
			"refusing to guess; pass the web id explicitly (see `geni profile unions`)",
			id, len(matches), profileArg, strings.Join(matches, ", "))
	}
}

// pickWebUnions returns the distinct web ids whose union can be the one
// described by the given partner and child guids. Split out from
// matchOAuthUnion so the matching rule is testable without an API client.
func pickWebUnions(webUnions []webtreeconflicts.WebUnion,
	wantPartners, wantChildren []string) []string {
	seen := make(map[string]struct{}, len(webUnions))
	var matches []string
	for _, wu := range webUnions {
		// The tree view can return the same union more than once; without this
		// one union counts as several and the command refuses a resolution that
		// is in fact unambiguous.
		if _, dup := seen[wu.WebID]; dup {
			continue
		}
		if !equalMembers(normalizeMembers(wu.Children), wantChildren) {
			continue
		}
		if !partnersOverlap(normalizeMembers(wu.Partners), wantPartners) {
			continue
		}
		seen[wu.WebID] = struct{}{}
		matches = append(matches, wu.WebID)
	}
	return matches
}

// memberGuids resolves short profile ids to guids in one bulk call.
func memberGuids(ctx context.Context, c *geni.Client, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	profiles, err := c.Profile().GetBulk(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, pr := range profiles.Results {
		if pr.Guid != "" && pr.ID != "" {
			out[pr.ID] = pr.Guid
		}
	}
	return out, nil
}

// mapGuids projects short profile ids through the lookup, dropping unresolved.
func mapGuids(ids []string, guidOf map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if g, ok := guidOf[id]; ok {
			out = append(out, g)
		}
	}
	return out
}

// partnersOverlap reports whether the two partner sets can describe the same
// union. Equality is too strict: the tree view lists partner slots the OAuth
// API omits, so a real union routinely shows an extra placeholder. Sharing one
// real partner is the strongest claim the data supports; two empty sets (a
// childless union with no resolvable partner) count as overlapping so the
// children comparison decides.
func partnersOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	in := make(map[string]struct{}, len(a))
	for _, x := range a {
		in[x] = struct{}{}
	}
	for _, y := range b {
		if _, ok := in[y]; ok {
			return true
		}
	}
	return false
}

// normalizeMembers strips the "profile-" prefix, de-duplicates and sorts, so
// OAuth and tree-view membership lists are comparable.
func normalizeMembers(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimPrefix(strings.TrimPrefix(id, "profile-g"), "profile-")
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func equalMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resolveProfileGuid returns id unchanged when it is already a bare guid,
// or looks up the profile's guid via the OAuth API when given a
// profile-NNN id. Shared by the matches web commands, which key off
// guids but accept the friendlier profile-NNN form.
func resolveProfileGuid(ctx context.Context, g *globalOpts, id string) (string, error) {
	if !strings.HasPrefix(id, "profile-") {
		return id, nil
	}
	c, err := newClient(g)
	if err != nil {
		return "", err
	}
	p, err := c.Profile().Get(ctx, id)
	if err != nil {
		return "", err
	}
	if p.Guid == "" {
		return "", errors.New("profile has no guid")
	}
	return p.Guid, nil
}

// runConflictsList handles
//
//	geni conflicts list [-page N | -all] [-limit N]
//
// It paginates the merge-center data-conflicts list via the Web AJAX
// client. Output is a JSON array of conflict entries. Gated by
// ensureWebConsent.
func runConflictsList(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni conflicts list", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	page := fs.Int("page", 0, "1-based page number; ignored with -all")
	all := fs.Bool("all", false, "paginate until no next page")
	limit := fs.Int("limit", 0, "cap output rows after pagination (0 = no cap)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: geni conflicts list [flags] (no positional args)")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	cc := webconflicts.NewClient(wc)

	startPage := *page
	if startPage <= 0 {
		startPage = 1
	}

	var out []webconflicts.Conflict
	for p := startPage; ; p++ {
		res, err := cc.List(ctx, webconflicts.ListOptions{Page: p})
		if err != nil {
			return err
		}
		out = append(out, res.Conflicts...)
		if !*all || !res.HasNext {
			break
		}
		if *limit > 0 && len(out) >= *limit {
			break
		}
	}

	if *limit > 0 && len(out) > *limit {
		out = out[:*limit]
	}
	return render(g.stdout, out)
}

// runTreeConflictsList handles
//
//	geni tree-conflicts list [-collection C] [-page N | -all] [-limit N]
//
// It paginates the merge-center tree-conflicts list via the Web AJAX
// client. Output is a JSON array of tree-conflict entries, each with a
// tree_url ("Open tree") link — the list is read-only, as tree conflicts
// have no programmatic resolution. Gated by ensureWebConsent.
func runTreeConflictsList(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni tree-conflicts list", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	collection := fs.String("collection", "managed",
		"viewing mode: managed|relatives|followed|collaborators (empty for server default)")
	page := fs.Int("page", 0, "1-based page number; ignored with -all")
	all := fs.Bool("all", false, "paginate until no next page")
	limit := fs.Int("limit", 0, "cap output rows after pagination (0 = no cap)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: geni tree-conflicts list [flags] (no positional args)")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	cc := webtreeconflicts.NewClient(wc)

	startPage := *page
	if startPage <= 0 {
		startPage = 1
	}

	var out []webtreeconflicts.TreeConflict
	for p := startPage; ; p++ {
		res, err := cc.List(ctx, webtreeconflicts.ListOptions{Collection: *collection, Page: p})
		if err != nil {
			return err
		}
		out = append(out, res.Conflicts...)
		if !*all || !res.HasNext {
			break
		}
		if *limit > 0 && len(out) >= *limit {
			break
		}
	}

	if *limit > 0 && len(out) > *limit {
		out = out[:*limit]
	}
	return render(g.stdout, out)
}

// runTreeConflictsShow handles
//
//	geni tree-conflicts show <profile-id>
//
// It inspects one profile's tree conflict via the Merge Center tree view
// (the /flash/fetch_immediate_family AJAX endpoint) and renders the parsed
// conflict: the focus, the parent unions, the suspected duplicate relatives,
// and runnable `geni profile compare`/`merge` suggestions. The argument is
// the bare id/guid from `tree-conflicts list`. Gated by ensureWebConsent.
func runTreeConflictsShow(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni tree-conflicts show", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni tree-conflicts show <profile-id>")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}

	detail, err := webtreeconflicts.NewClient(wc).Show(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	return render(g.stdout, detail)
}

// runTreeConflictsResolve handles
//
//	geni tree-conflicts resolve [-yes] [-dry-run] <profile-id>
//
// It resolves the "empty parent slot" variant of a duplicate_parents tree
// conflict: the focus child has one real parent union and one union whose
// second parent is a synthetic «Mother/Father of X» placeholder (minted for a
// record that named a single parent). The placeholder is not a real profile —
// a `profile merge` against it 404s — so the conflict is resolved by detaching
// the focus from the placeholder union (delete_relationships), leaving the real
// family union intact. Other duplicate_parents (two real parents) and
// duplicate_spouse conflicts are not auto-resolvable here; resolve reports the
// merge commands from `show` for those. Mutating, so a y/N confirmation is
// required unless -yes; -dry-run reports the plan without detaching. Gated by
// the one-time AJAX consent prompt.
func runTreeConflictsResolve(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni tree-conflicts resolve", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	dryRun := fs.Bool("dry-run", false, "report the plan without detaching")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni tree-conflicts resolve [-yes] [-dry-run] <profile-id>")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}

	detail, err := webtreeconflicts.NewClient(wc).Show(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if !detail.HasConflict {
		return render(g.stdout, map[string]any{
			"status":  "no_conflict",
			"profile": detail.Focus.ProfileID,
		})
	}

	unionIDs := detail.EmptyParentUnions()
	if len(unionIDs) == 0 {
		// Not the empty-slot case — two real parents (or a spouse conflict).
		return render(g.stdout, map[string]any{
			"status":            "manual",
			"profile":           detail.Focus.ProfileID,
			"message":           "not an empty-parent-slot conflict — resolve by merging the duplicate relatives (see suggested_actions)",
			"suggested_actions": detail.SuggestedActions,
		})
	}

	if *dryRun {
		return render(g.stdout, map[string]any{
			"status":                   "dry_run",
			"profile":                  detail.Focus.ProfileID,
			"action":                   "detach_empty_parent_union",
			"would_detach_from_unions": unionIDs,
		})
	}

	if !*yes {
		_, _ = fmt.Fprintf(g.stderr,
			"Resolve tree conflict: detach %s from empty-parent union(s) %s? [y/N]: ",
			detail.Focus.ProfileID, strings.Join(unionIDs, ", "))
		if !confirmed(g.stdin) {
			return errors.New("resolve aborted")
		}
	}

	if err := webunions.NewClient(wc).Detach(ctx, detail.Focus.ProfileID, unionIDs); err != nil {
		return err
	}
	return render(g.stdout, map[string]any{
		"status":               "resolved",
		"profile":              detail.Focus.ProfileID,
		"action":               "detach_empty_parent_union",
		"detached_from_unions": unionIDs,
	})
}

// runConflictsShow handles
//
//	geni conflicts show <profile-id-or-guid>
//
// It fetches /merge/resolve/<guid> and reports the conflicting fields.
// A profile with no outstanding conflict prints has_conflict:false.
// Accepts either a profile-NNN id (resolved to a guid via the OAuth API)
// or a bare guid. Gated by ensureWebConsent.
func runConflictsShow(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni conflicts show", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni conflicts show <profile-id-or-guid>")
	}

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	guid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	res, err := webconflicts.NewClient(wc).Get(ctx, guid)
	if err != nil {
		return err
	}
	return render(g.stdout, res)
}

// runConflictsResolve handles
//
//	geni conflicts resolve [-yes] [-prefer-nonempty] [-pick field=col]... [-dry-run] <profile-id-or-guid>
//
// By default it clears a profile's merge data conflict by keeping the
// surviving (primary) profile's value for every conflicting field — the
// correct default when the survivor is canonical. -prefer-nonempty instead
// keeps a merged-in (e.g. external contributor's) value for any field the
// survivor left blank, so that contribution is not silently dropped. -pick
// resolves a named field to an explicit column (0 = primary, 1+ = a merged
// profile's value). -dry-run fetches the conflict and prints the choices it
// would submit without changing anything.
//
// The action is destructive (it overwrites the merge's unresolved state),
// so a y/N confirmation is required unless -yes (or -dry-run) is passed.
// Resolving an already-clean profile is a no-op. Accepts a profile-NNN id or
// a bare guid. Gated by ensureWebConsent.
func runConflictsResolve(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni conflicts resolve", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	preferNonEmpty := fs.Bool("prefer-nonempty", false,
		"keep a merged-in value for any field the survivor left blank")
	dryRun := fs.Bool("dry-run", false,
		"print the choices that would be submitted without resolving")
	var picks repeatableFlag
	fs.Var(&picks, "pick",
		"resolve a field to an explicit column: -pick field=col (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: geni conflicts resolve [-yes] [-prefer-nonempty] [-pick field=col]... [-dry-run] <profile-id-or-guid>")
	}
	pickMap, err := parsePicks(picks)
	if err != nil {
		return err
	}
	// "Smart" resolution (per-field choices) needs the conflict's fields and
	// blobs up front; plain keep-primary lets Resolve fetch them itself.
	smart := *preferNonEmpty || len(pickMap) > 0 || *dryRun

	if err := ensureWebConsent(g); err != nil {
		return err
	}
	guid, err := resolveProfileGuid(ctx, g, fs.Arg(0))
	if err != nil {
		return err
	}

	cookies, err := loadWebCookies(g)
	if err != nil {
		return err
	}
	wc, err := newWebClient(g, cookies)
	if err != nil {
		return err
	}
	cc := webconflicts.NewClient(wc)

	var choices map[string]string
	if smart {
		detail, err := cc.Get(ctx, guid)
		if err != nil {
			return err
		}
		if !detail.HasConflict {
			return render(g.stdout, map[string]string{
				"status":  "no-conflict",
				"profile": guid,
			})
		}
		choices, err = webconflicts.BuildResolveChoices(detail.Fields, *preferNonEmpty, pickMap)
		if err != nil {
			return err
		}
		if *dryRun {
			return render(g.stdout, dryRunResolution(guid, detail.Fields, choices))
		}
	}

	if !*yes {
		_, _ = fmt.Fprintf(g.stderr,
			"Resolve data conflict for profile %s? [y/N]: ", guid)
		if !confirmed(g.stdin) {
			return errors.New("resolve aborted")
		}
	}

	if err := cc.Resolve(ctx, guid, choices); err != nil {
		return err
	}
	return render(g.stdout, map[string]string{
		"status":  "resolved",
		"profile": guid,
	})
}

// repeatableFlag collects a flag that may appear more than once.
type repeatableFlag []string

func (r *repeatableFlag) String() string { return strings.Join(*r, ",") }
func (r *repeatableFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// parsePicks turns "field=col" entries into a field → column-index map. An
// empty input yields an empty (non-nil) map, never nil.
func parsePicks(raw []string) (map[string]int, error) {
	out := make(map[string]int, len(raw))
	for _, p := range raw {
		field, col, ok := strings.Cut(p, "=")
		if !ok || field == "" {
			return nil, fmt.Errorf("pick %q: want field=col", p)
		}
		idx, err := strconv.Atoi(strings.TrimSpace(col))
		if err != nil {
			return nil, fmt.Errorf("pick %q: column must be an integer", p)
		}
		out[field] = idx
	}
	return out, nil
}

// dryRunResolution renders, per conflicting field, the value that would be
// submitted: the chosen merged-in value where a choice was made, otherwise
// the surviving profile's primary value (the keep-primary default).
func dryRunResolution(guid string, fields []webconflicts.ConflictField, choices map[string]string) map[string]any {
	rows := make([]map[string]string, 0, len(fields))
	for _, f := range fields {
		row := map[string]string{"field": f.Field, "primary": f.PrimaryValue}
		if blob, ok := choices[f.Field]; ok {
			row["action"] = "keep-merged"
			row["chosen"] = displayForBlob(f, blob)
		} else {
			row["action"] = "keep-primary"
			row["chosen"] = f.PrimaryValue
		}
		rows = append(rows, row)
	}
	return map[string]any{
		"status":  "dry-run",
		"profile": guid,
		"fields":  rows,
	}
}

// displayForBlob maps a chosen submit blob back to its displayed text by
// its column position, for human-readable dry-run output.
func displayForBlob(f webconflicts.ConflictField, blob string) string {
	for i, b := range f.DataResolveData {
		if b == blob && i < len(f.DisplayValues) {
			return f.DisplayValues[i]
		}
	}
	return ""
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
