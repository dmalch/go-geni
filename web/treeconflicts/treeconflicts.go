// Package treeconflicts exposes the website-only "tree conflicts" feature
// — the Merge Center tab that lists profiles which, after a merge, may
// have ended up with duplicate close relatives and need a human to review
// the tree. It is the sibling of the "data conflicts" list (see
// web/conflicts): both are server-rendered Merge Center pages the OAuth
// API has no equivalent for, so this package GETs the HTML and parses it,
// mirroring web/conflicts and web/matches. See the parent package doc for
// legal caveats.
//
// Unlike data conflicts, a tree conflict has NO programmatic resolution:
// the web UI's only per-row action is "Open tree" (a navigation link to
// /family-tree/index/<id>?resolve=<id>), and resolving it is a manual
// visual task. So this package is list-only — there is no Get/Resolve.
//
// The list is per-user and not reproducible without an account that owns
// merged profiles with pending tree conflicts, so there is no acceptance
// test for it.
package treeconflicts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
