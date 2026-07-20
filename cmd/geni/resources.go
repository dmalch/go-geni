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
	"strconv"
	"strings"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/tree"
	webconflicts "github.com/dmalch/go-geni/web/conflicts"
	webdocument "github.com/dmalch/go-geni/web/document"
	webmatches "github.com/dmalch/go-geni/web/matches"
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
// The union arguments must be Geni web ids (the bare 6000000… numbers
// shown as remove_connection_<id> on the edit_relationships page),
// optionally "union-"-prefixed. The OAuth API exposes no union guid, so
// a short union-NNN cannot be resolved to a web id here.
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
