// Package treeconflicts exposes the website-only "tree conflicts" feature
// — the Merge Center tab that lists profiles which, after a merge, may
// have ended up with duplicate close relatives and need a human to review
// the tree. It is the sibling of the "data conflicts" list (see
// web/conflicts): both are server-rendered Merge Center pages the OAuth
// API has no equivalent for, so this package GETs the HTML and parses it,
// mirroring web/conflicts and web/matches. See the parent package doc for
// legal caveats.
//
// Most tree conflicts are resolved by merging the duplicate relatives (two
// father profiles, two mother profiles) — Show emits those merge commands.
// The one variant this package can resolve directly is the "empty parent
// slot": a duplicate_parents conflict where one parent union's second parent
// is a synthetic «Mother/Father of X» placeholder (minted for a record that
// named a single parent). That placeholder is not a real profile — a merge
// 404s — so EmptyParentUnions surfaces the placeholder union's web id and the
// focus is detached from it (via web/unions), leaving the real family union.
// List enumerates the conflicts; Show inspects one, reproducing Geni's own
// tree-view analysis (/flash/fetch_immediate_family) to surface the parent
// unions (each with its web id), the suspected duplicate relatives, and
// ready-to-run compare/merge/resolve commands.
//
// The data is per-user and not reproducible without an account that owns
// merged profiles with pending tree conflicts, so there is no acceptance
// test for it.
package treeconflicts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/dmalch/go-geni/web"
)

// ListOptions controls the /list/tree_conflicts GET parameters. Page is
// 1-based; 0 means "default" (page 1). Collection selects the "Viewing
// mode" dropdown (managed|relatives|followed|collaborators); "" leaves it
// to the server default.
type ListOptions struct {
	Collection string
	Page       int
}

// TreeConflict is one row of the tree-conflicts list — a merged profile
// that may now carry duplicate close relatives. The authoritative data is
// the ProfileID and the TreeURL; name/actor/manager/date are best-effort
// enrichment from the table cells.
type TreeConflict struct {
	ProfileID     string `json:"profile_id"`
	Name          string `json:"name,omitempty"`
	ProfileURL    string `json:"profile_url,omitempty"`
	UpdatedByName string `json:"updated_by_name,omitempty"`
	UpdatedAtText string `json:"updated_at_text,omitempty"`
	ManagerName   string `json:"manager_name,omitempty"`
	TreeURL       string `json:"tree_url"`
}

// ListResult is one page of tree conflicts plus pagination state.
type ListResult struct {
	Conflicts []TreeConflict `json:"conflicts"`
	Page      int            `json:"page"`
	HasNext   bool           `json:"has_next"`
}

// Client wraps a *web.Client with the tree-conflicts endpoint.
type Client struct {
	web *web.Client
}

// NewClient returns a tree-conflicts Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// List fetches one page of the tree-conflicts (Merge Center) list.
func (c *Client) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	u := c.web.BaseURL() + "/list/tree_conflicts"
	q := url.Values{}
	if opts.Collection != "" {
		q.Set("collection", opts.Collection)
	}
	if opts.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.web.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list/tree_conflicts: HTTP %d", resp.StatusCode)
	}

	currentPage := opts.Page
	if currentPage <= 0 {
		currentPage = 1
	}
	return parseListConflicts(resp.Body, currentPage)
}

// ConflictProfile is one relative involved in a tree conflict, projected
// from the /flash/fetch_immediate_family node shape into stable names.
type ConflictProfile struct {
	ProfileID   string `json:"profile_id"` // pr_id — the guid used across the API
	ShortID     string `json:"short_id"`   // pid — the internal id used by prune counts
	Name        string `json:"name,omitempty"`
	Gender      string `json:"gender,omitempty"` // "male" | "female" | ""
	BirthYear   string `json:"birth_year,omitempty"`
	DeathYear   string `json:"death_year,omitempty"`
	SubtreeSize string `json:"subtree_size,omitempty"` // e.g. "+110", from fetch_prune_counts
	// Placeholder marks a synthetic empty-parent-slot node — «Mother/Father of
	// X», minted for a record that named a single parent. It is NOT a real
	// profile (it has no short id and a negative pseudo-guid), so a
	// `profile merge` against it 404s; the fix is a detach (see Resolve).
	Placeholder bool `json:"placeholder,omitempty"`
}

// ParentUnion is one of the focus profile's parent unions (a union in
// which the focus is a child). A tree conflict typically has two of these.
type ParentUnion struct {
	UnionID string `json:"union_id"` // tree-session-local label ("union-<u>")
	// WebUnionID is the union's Geni web id (bare 6000000…, from the flash
	// `m` field) — the id `profile detach-union` / delete_relationships take.
	WebUnionID string            `json:"web_union_id,omitempty"`
	Parents    []ConflictProfile `json:"parents"`
}

// hasPlaceholderParent reports whether any partner of the union is a synthetic
// empty slot (so the union is a duplicate "empty-parent" union, not a real one).
func (pu ParentUnion) hasPlaceholderParent() bool {
	for _, p := range pu.Parents {
		if p.Placeholder {
			return true
		}
	}
	return false
}

// DuplicateCandidate groups two or more same-role relatives that look like
// the same person duplicated by a merge — the profiles to compare/merge.
type DuplicateCandidate struct {
	Role     string            `json:"role"` // father | mother | spouse
	Profiles []ConflictProfile `json:"profiles"`
}

// ConflictDetail is the parsed, analyzed tree conflict for one profile.
type ConflictDetail struct {
	ProfileID           string               `json:"profile_id"`
	Focus               ConflictProfile      `json:"focus"`
	ConflictTypes       []string             `json:"conflict_types"`     // e.g. ["duplicate_parents"]
	ParentUnionCount    int                  `json:"parent_union_count"` // Geni's npu marker
	PartnerConflict     bool                 `json:"partner_conflict"`   // Geni's ptc marker
	ParentUnions        []ParentUnion        `json:"parent_unions"`
	DuplicateCandidates []DuplicateCandidate `json:"duplicate_candidates"`
	SuggestedActions    []string             `json:"suggested_actions"`
	HasConflict         bool                 `json:"has_conflict"`
}

// flash* mirror the compact /flash/fetch_immediate_family JSON.
type flashResponse struct {
	Tree flashTree `json:"tree"`
}

type flashTree struct {
	Unions []flashUnion `json:"unions"`
	Nodes  []flashNode  `json:"nodes"`
}

type flashUnion struct {
	U int    `json:"u"` // union id local to this response
	M string `json:"m"` // the union's Geni web id (bare 6000000… — the delete_relationships uid)
	P []int  `json:"p"` // partner node ids
	C []int  `json:"c"` // child node ids
}

type flashNode struct {
	N     int    `json:"n"`     // node id local to this response
	PID   string `json:"pid"`   // short profile id
	PrID  string `json:"pr_id"` // full guid
	G     string `json:"g"`     // "m" | "f"
	NM    string `json:"nm"`    // display name
	DobY  string `json:"dob_y"`
	DodY  string `json:"dod_y"`
	Focus int    `json:"focus"` // 1 on the subject node
	NPU   string `json:"npu"`   // number of parent unions (wire type is a string)
	PTC   int    `json:"ptc"`   // partner-conflict flag
}

// Show fetches and analyzes one profile's tree conflict from the Merge
// Center's tree view. profileID is the bare id/guid from List (a leading
// "profile-"/"profile-g" is tolerated). It bootstraps a treeSessionId from
// the tree page, GETs /flash/fetch_immediate_family with the conflict
// probes, classifies the conflict, and (best-effort) annotates the
// duplicate candidates with their subtree sizes.
func (c *Client) Show(ctx context.Context, profileID string) (*ConflictDetail, error) {
	profileID = normalizeFlashProfileID(profileID)

	sid, err := c.fetchTreeSessionID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	tree, err := c.fetchImmediateFamily(ctx, sid, profileID)
	if err != nil {
		return nil, err
	}

	detail := analyzeConflict(profileID, tree)

	if pids := detail.candidateShortIDs(); len(pids) > 0 {
		if sizes, err := c.fetchPruneCounts(ctx, sid, profileID, pids); err == nil {
			detail.applySubtreeSizes(sizes)
		}
	}
	detail.buildSuggestedActions()
	return detail, nil
}

// normalizeFlashProfileID strips a "profile-g"/"profile-" prefix so the
// bare id the /flash endpoints expect is passed through.
func normalizeFlashProfileID(id string) string {
	id = strings.TrimPrefix(id, "profile-g")
	id = strings.TrimPrefix(id, "profile-")
	return id
}

// fetchTreeSessionID GETs the tree page and extracts the treeSessionId the
// /flash endpoints require. A not-logged-in session surfaces as
// web.ErrNotLoggedIn from the web client.
func (c *Client) fetchTreeSessionID(ctx context.Context, profileID string) (string, error) {
	u := c.web.BaseURL() + "/family-tree/index/" + profileID + "?resolve=" + url.QueryEscape(profileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.web.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("family-tree/index: HTTP %d", resp.StatusCode)
	}
	sid, err := parseTreeSessionID(resp.Body)
	if err != nil {
		return "", err
	}
	if sid == "" {
		return "", fmt.Errorf("tree session id not found for profile %s", profileID)
	}
	return sid, nil
}

// fetchImmediateFamily GETs the conflict-probing immediate-family JSON.
func (c *Client) fetchImmediateFamily(ctx context.Context, sid, profileID string) (*flashTree, error) {
	q := url.Values{
		"treeSessionId":           {sid},
		"profile":                 {profileID},
		"resolve_duplicates":      {"true"},
		"check_partner_conflicts": {"true"},
		"format":                  {"json"},
	}
	u := c.web.BaseURL() + "/flash/fetch_immediate_family?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := c.web.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch_immediate_family: HTTP %d", resp.StatusCode)
	}
	var fr flashResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return nil, fmt.Errorf("fetch_immediate_family: decode: %w", err)
	}
	return &fr.Tree, nil
}

// fetchPruneCounts annotates the given short profile ids with their subtree
// sizes (how many nodes hang off each) — used to pick the merge direction.
func (c *Client) fetchPruneCounts(ctx context.Context, sid, profileID string, shortIDs []string) (map[string]string, error) {
	q := url.Values{
		"treeSessionId": {sid},
		"profile":       {profileID},
		"target_ids":    {strings.Join(shortIDs, ",")},
		"format":        {"json"},
	}
	u := c.web.BaseURL() + "/flash/fetch_prune_counts?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := c.web.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch_prune_counts: HTTP %d", resp.StatusCode)
	}
	var pc struct {
		PruneCounts []struct {
			PID json.Number `json:"pid"`
			P   string      `json:"p"`
		} `json:"prune_counts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pc); err != nil {
		return nil, fmt.Errorf("fetch_prune_counts: decode: %w", err)
	}
	out := make(map[string]string, len(pc.PruneCounts))
	for _, e := range pc.PruneCounts {
		out[e.PID.String()] = e.P
	}
	return out, nil
}

// analyzeConflict turns the raw immediate-family tree into a ConflictDetail:
// it finds the focus, classifies the focus's unions into parent unions (the
// focus is a child) vs own unions (the focus is a partner), and groups
// same-role relatives that look duplicated. It is pure — no HTTP — so the
// classification is unit-testable on its own.
func analyzeConflict(profileID string, t *flashTree) *ConflictDetail {
	d := &ConflictDetail{
		ProfileID:           profileID,
		ConflictTypes:       []string{},
		ParentUnions:        []ParentUnion{},
		DuplicateCandidates: []DuplicateCandidate{},
		SuggestedActions:    []string{},
	}

	byN := make(map[int]*flashNode, len(t.Nodes))
	var focus *flashNode
	for i := range t.Nodes {
		byN[t.Nodes[i].N] = &t.Nodes[i]
		if t.Nodes[i].Focus != 0 {
			focus = &t.Nodes[i]
		}
	}
	if focus == nil {
		return d
	}
	d.Focus = toConflictProfile(focus)
	d.PartnerConflict = focus.PTC != 0
	if n, err := strconv.Atoi(focus.NPU); err == nil {
		d.ParentUnionCount = n
	}

	var parentUnions, ownUnions []flashUnion
	for _, u := range t.Unions {
		switch {
		case slices.Contains(u.C, focus.N):
			parentUnions = append(parentUnions, u)
		case slices.Contains(u.P, focus.N):
			ownUnions = append(ownUnions, u)
		}
	}
	if d.ParentUnionCount == 0 {
		d.ParentUnionCount = len(parentUnions)
	}

	for _, u := range parentUnions {
		pu := ParentUnion{UnionID: fmt.Sprintf("union-%d", u.U), WebUnionID: u.M}
		for _, pn := range u.P {
			if n := byN[pn]; n != nil {
				pu.Parents = append(pu.Parents, toConflictProfile(n))
			}
		}
		d.ParentUnions = append(d.ParentUnions, pu)
	}

	// Duplicate parents: a person has exactly one father and one mother, so
	// two same-gender parents across the parent unions are the duplicates.
	if len(parentUnions) >= 2 {
		d.ConflictTypes = append(d.ConflictTypes, "duplicate_parents")
		byGender := map[string][]ConflictProfile{}
		for _, pu := range d.ParentUnions {
			for _, p := range pu.Parents {
				byGender[p.Gender] = append(byGender[p.Gender], p)
			}
		}
		for _, gender := range []string{"male", "female"} { // deterministic: father, then mother
			if grp := byGender[gender]; len(grp) >= 2 {
				d.DuplicateCandidates = append(d.DuplicateCandidates,
					DuplicateCandidate{Role: parentRole(gender), Profiles: grp})
			}
		}
	}

	// Duplicate spouse: the focus appears as a partner in more than one union.
	if len(ownUnions) >= 2 {
		var spouses []ConflictProfile
		for _, u := range ownUnions {
			for _, pn := range u.P {
				if pn == focus.N {
					continue
				}
				if n := byN[pn]; n != nil {
					spouses = append(spouses, toConflictProfile(n))
				}
			}
		}
		if len(spouses) >= 2 {
			d.ConflictTypes = append(d.ConflictTypes, "duplicate_spouse")
			d.DuplicateCandidates = append(d.DuplicateCandidates,
				DuplicateCandidate{Role: "spouse", Profiles: spouses})
		}
	}

	d.HasConflict = len(d.ConflictTypes) > 0
	return d
}

// candidateShortIDs returns the short ids of every duplicate-candidate
// profile, for a fetch_prune_counts lookup.
func (d *ConflictDetail) candidateShortIDs() []string {
	var ids []string
	seen := map[string]bool{}
	for _, cand := range d.DuplicateCandidates {
		for _, p := range cand.Profiles {
			if p.ShortID != "" && !seen[p.ShortID] {
				ids = append(ids, p.ShortID)
				seen[p.ShortID] = true
			}
		}
	}
	return ids
}

// applySubtreeSizes copies fetch_prune_counts results onto the duplicate
// candidates (keyed by short id).
func (d *ConflictDetail) applySubtreeSizes(sizes map[string]string) {
	for ci := range d.DuplicateCandidates {
		for pi := range d.DuplicateCandidates[ci].Profiles {
			p := &d.DuplicateCandidates[ci].Profiles[pi]
			// Skip meaningless values (Geni returns a bare "+" for zero).
			if s, ok := sizes[p.ShortID]; ok && strings.Trim(s, "+ ") != "" {
				p.SubtreeSize = s
			}
		}
	}
}

// EmptyParentUnions returns the web ids of the focus's parent unions whose
// second parent is a synthetic empty slot — the duplicate "empty-parent" unions
// to detach the focus from, resolving a placeholder duplicate_parents conflict.
// It returns nil unless at least one FULLY-REAL parent union remains (so the
// focus stays parented) — never detaching the focus from every parent union.
func (d *ConflictDetail) EmptyParentUnions() []string {
	var empty []string
	realCount := 0
	for _, pu := range d.ParentUnions {
		if pu.hasPlaceholderParent() {
			if pu.WebUnionID != "" {
				empty = append(empty, pu.WebUnionID)
			}
		} else {
			realCount++
		}
	}
	if realCount == 0 {
		return nil
	}
	return empty
}

// candidateHasPlaceholder reports whether a duplicate candidate includes a
// synthetic empty-slot node (so it cannot be resolved by a profile merge).
func candidateHasPlaceholder(ps []ConflictProfile) bool {
	for _, p := range ps {
		if p.Placeholder {
			return true
		}
	}
	return false
}

// buildSuggestedActions emits runnable geni commands for each duplicate
// candidate, keeping the profile with the larger attached subtree. A candidate
// that includes a synthetic empty slot is not mergeable (a merge 404s), so it
// suggests `tree-conflicts resolve` (a detach) instead.
func (d *ConflictDetail) buildSuggestedActions() {
	for _, cand := range d.DuplicateCandidates {
		if len(cand.Profiles) < 2 {
			continue
		}
		if candidateHasPlaceholder(cand.Profiles) {
			d.SuggestedActions = append(d.SuggestedActions,
				fmt.Sprintf("geni tree-conflicts resolve profile-g%s  # %s: detach the empty %s slot (placeholder is not a mergeable profile)",
					d.Focus.ProfileID, cand.Role, cand.Role))
			continue
		}
		ps := make([]ConflictProfile, len(cand.Profiles))
		copy(ps, cand.Profiles)
		sort.SliceStable(ps, func(i, j int) bool {
			return subtreeValue(ps[i].SubtreeSize) > subtreeValue(ps[j].SubtreeSize)
		})
		keep, dup := ps[0], ps[1]
		d.SuggestedActions = append(d.SuggestedActions,
			fmt.Sprintf("geni profile compare profile-g%s profile-g%s", keep.ProfileID, dup.ProfileID),
			fmt.Sprintf("geni profile merge profile-g%s profile-g%s  # %s: keep %s",
				keep.ProfileID, dup.ProfileID, cand.Role, keep.Name),
		)
	}
}

// treeSessionIDPattern matches the treeSessionId the tree page embeds. It
// appears as a JS property in an inline <script> (treeSessionId: "<hex>"),
// so a plain regex is more robust than HTML parsing — and it must key off
// the "treeSessionId" label because the page also carries an unrelated
// 64-hex session token elsewhere. The [:=] tolerates both the JS colon and
// an HTML attribute form.
var treeSessionIDPattern = regexp.MustCompile(`(?i)treeSessionId\s*[:=]\s*["']([0-9a-f]{16,})["']`)

// parseTreeSessionID extracts the treeSessionId from the tree page body.
func parseTreeSessionID(r io.Reader) (string, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if m := treeSessionIDPattern.FindSubmatch(body); m != nil {
		return string(m[1]), nil
	}
	return "", nil
}

func toConflictProfile(n *flashNode) ConflictProfile {
	return ConflictProfile{
		ProfileID:   n.PrID,
		ShortID:     n.PID,
		Name:        trimText(n.NM),
		Gender:      genderName(n.G),
		BirthYear:   n.DobY,
		DeathYear:   n.DodY,
		Placeholder: isPlaceholderNode(n),
	}
}

// isPlaceholderNode reports whether a node is a synthetic empty-parent slot:
// Geni gives it no short id (pid) and a negative pseudo-guid (pr_id "-<n>f").
func isPlaceholderNode(n *flashNode) bool {
	return n.PID == "" || strings.HasPrefix(n.PrID, "-")
}

func genderName(g string) string {
	switch g {
	case "m":
		return "male"
	case "f":
		return "female"
	default:
		return ""
	}
}

func parentRole(gender string) string {
	switch gender {
	case "male":
		return "father"
	case "female":
		return "mother"
	default:
		return "parent"
	}
}

// subtreeValue parses a fetch_prune_counts size like "+110" into 110; an
// unknown/empty size sorts last (-1) so a known-larger subtree is kept.
func subtreeValue(s string) int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "+")
	if s == "" {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

// parseListConflicts walks the tree-conflicts HTML page and returns the
// rows plus a HasNext flag.
func parseListConflicts(r io.Reader, currentPage int) (*ListResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	out := &ListResult{Conflicts: []TreeConflict{}, Page: currentPage}
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "tr":
			if id := attr(n, "data-profile-id"); id != "" {
				out.Conflicts = append(out.Conflicts, parseConflictRow(n, id))
			}
		case "ul":
			if hasClass(n, "pagination") {
				out.HasNext = paginationHasNext(n, currentPage)
			}
		}
		return true
	})
	return out, nil
}

// parseConflictRow extracts a single <tr data-profile-id="..."> row into a
// TreeConflict. The authoritative data is the id and the /family-tree/index
// URL; name/actor/manager/date are best-effort.
func parseConflictRow(tr *html.Node, id string) TreeConflict {
	tc := TreeConflict{ProfileID: id}
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		switch {
		case hasClass(c, "name-area-grid"):
			tc.Name, tc.ProfileURL = firstLink(c)
		case hasClass(c, "actor-area-grid"):
			tc.UpdatedByName, _ = firstLink(c)
		case hasClass(c, "manager-area-grid"):
			tc.ManagerName, _ = firstLink(c)
		case hasClass(c, "update-on-area-grid"):
			tc.UpdatedAtText = trimText(textWithoutAreaTitle(c))
		case hasClass(c, "action-area-grid"):
			tc.TreeURL = treeHref(c)
		}
	}
	return tc
}

// firstLink returns the text and href of the first <a> in n.
func firstLink(n *html.Node) (text, href string) {
	walk(n, func(x *html.Node) bool {
		if text != "" {
			return false
		}
		if x.Type == html.ElementNode && x.Data == "a" {
			href = attr(x, "href")
			text = trimText(textOf(x))
			return false
		}
		return true
	})
	return text, href
}

// treeHref returns the first /family-tree/index/<id> link in n — the row's
// "Open tree" action.
func treeHref(n *html.Node) string {
	var found string
	walk(n, func(x *html.Node) bool {
		if found != "" {
			return false
		}
		if x.Type == html.ElementNode && x.Data == "a" {
			if href := attr(x, "href"); strings.HasPrefix(href, "/family-tree/index/") {
				found = href
				return false
			}
		}
		return true
	})
	return found
}

// paginationHasNext returns true if the pagination block links to
// currentPage+1.
func paginationHasNext(ul *html.Node, currentPage int) bool {
	want := fmt.Sprintf("page=%d", currentPage+1)
	found := false
	walk(ul, func(n *html.Node) bool {
		if found {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		if strings.Contains(attr(n, "href"), want) {
			found = true
			return false
		}
		return true
	})
	return found
}

// trimText collapses runs of whitespace in s and trims the ends. Geni's
// HTML carries heavy indent whitespace that needs squeezing.
func trimText(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// textOf concatenates all descendant text nodes of n.
func textOf(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) bool {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		return true
	})
	return b.String()
}

// textWithoutAreaTitle is textOf but skips any <div class="area-title">
// subtree, which carries a localized header label ("Менеджер:",
// "Обновлено:") that is not part of the value.
func textWithoutAreaTitle(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if x.Type == html.ElementNode && x.Data == "div" && hasClass(x, "area-title") {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return b.String()
}

func walk(n *html.Node, visit func(*html.Node) bool) bool {
	if !visit(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !walk(c, visit) {
			return false
		}
	}
	return true
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, want string) bool {
	for c := range strings.FieldsSeq(attr(n, "class")) {
		if c == want {
			return true
		}
	}
	return false
}
