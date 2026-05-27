// Package matches exposes the website-only Merge Center list. The
// OAuth API has no equivalent — /list/matches is a server-rendered
// HTML page (no JSON endpoint, no AJAX call), so this package GETs
// the page and parses the row table. See the parent package doc for
// legal caveats.
//
// The list is per-user and not reproducible without an account that
// owns pending matches, so there is no acceptance test for it.
package matches

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/dmalch/go-geni/web"
)

// Collection scopes the listing by whose profiles to include.
type Collection string

const (
	CollectionManaged       Collection = "managed"       // "Управляется мной"
	CollectionRelatives     Collection = "relatives"     // "Мои родственники"
	CollectionFollowed      Collection = "followed"      // "Мои подписки"
	CollectionCollaborators Collection = "collaborators" // "Управляется моими соавторами"
)

// Filter restricts the listing to one match type.
type Filter string

const (
	FilterTreeMatches   Filter = "tree_matches"
	FilterRecordMatches Filter = "record_matches"
	FilterSmartMatches  Filter = "smart_matches"
)

// Order picks the sort column.
type Order string

const (
	OrderName         Order = "last_or_maiden_name"
	OrderRelationship Order = "relationship"
	OrderManager      Order = "manager"
	OrderUpdatedAt    Order = "mc_updated_at"
	OrderMatches      Order = "value_add"
)

// Direction is the sort direction.
type Direction string

const (
	DirectionAsc  Direction = "asc"
	DirectionDesc Direction = "desc"
)

// ListOptions controls the /list/matches GET parameters. All fields
// are optional; zero values are omitted and let the server pick its
// defaults.
type ListOptions struct {
	Collection Collection
	Filter     Filter
	Order      Order
	Direction  Direction
	Page       int // 1-based; 0 means "default" (page 1).
}

// Match is one row of the merge-center table.
type Match struct {
	ProfileGuid       string `json:"profile_guid"`
	Name              string `json:"name"`
	ProfileURL        string `json:"profile_url"`
	LifespanText      string `json:"lifespan_text,omitempty"`
	Deceased          bool   `json:"deceased,omitempty"`
	Privacy           string `json:"privacy,omitempty"`
	Relationship      string `json:"relationship,omitempty"`
	ManagerName       string `json:"manager_name,omitempty"`
	ManagerProfileURL string `json:"manager_profile_url,omitempty"`
	UpdatedAtText     string `json:"updated_at_text,omitempty"`
	TreeMatchCount    int    `json:"tree_match_count"`
	RecordMatchCount  int    `json:"record_match_count"`
	SmartMatchCount   int    `json:"smart_match_count"`
	SmartMatchValue   int    `json:"smart_match_value,omitempty"`
}

// ListResult is one page of matches plus pagination state.
type ListResult struct {
	Matches []Match `json:"matches"`
	Page    int     `json:"page"`
	HasNext bool    `json:"has_next"`
}

// Client wraps a *web.Client with the matches endpoints.
type Client struct {
	web *web.Client
}

// NewClient returns a matches Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// List fetches one page of the merge-center matches list.
func (c *Client) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	q := url.Values{}
	if opts.Collection != "" {
		q.Set("collection", string(opts.Collection))
	}
	if opts.Filter != "" {
		q.Set("filter", string(opts.Filter))
	}
	if opts.Order != "" {
		q.Set("order", string(opts.Order))
	}
	if opts.Direction != "" {
		q.Set("direction", string(opts.Direction))
	}
	if opts.Page > 0 {
		q.Set("page", strconv.Itoa(opts.Page))
	}

	u := c.web.BaseURL() + "/list/matches"
	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
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
		return nil, fmt.Errorf("list/matches: HTTP %d", resp.StatusCode)
	}

	currentPage := opts.Page
	if currentPage <= 0 {
		currentPage = 1
	}
	return parseListMatches(resp.Body, currentPage)
}

// parseListMatches walks the merge-center HTML page and returns the
// rows plus a HasNext flag.
func parseListMatches(r io.Reader, currentPage int) (*ListResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	out := &ListResult{Matches: []Match{}, Page: currentPage}
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "tr":
			if guid := attr(n, "data-profile-id"); guid != "" {
				out.Matches = append(out.Matches, parseRow(n, guid))
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

// parseRow extracts a single <tr data-profile-id="..."> into a Match.
func parseRow(tr *html.Node, guid string) Match {
	m := Match{
		ProfileGuid: guid,
		Deceased:    attr(tr, "data-deceased") == "true",
		Privacy:     attr(tr, "data-privacy"),
	}
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		switch {
		case hasClass(c, "name-area-grid"):
			parseNameCell(c, &m)
		case hasClass(c, "relationship-area-grid"):
			m.Relationship = trimText(textOf(c))
		case hasClass(c, "manager-area-grid"):
			parseManagerCell(c, &m)
		case hasClass(c, "update-on-area-grid"):
			m.UpdatedAtText = trimText(textWithoutAreaTitle(c))
		case hasClass(c, "buttons-area-grid"):
			parseButtonsCell(c, &m)
		}
	}
	return m
}

func parseNameCell(td *html.Node, m *Match) {
	walk(td, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "a":
			if m.Name == "" && hasClass(n, "strong") {
				m.ProfileURL = attr(n, "href")
				m.Name = trimText(textOf(n))
			}
		case "span":
			if hasClass(n, "quiet") && m.LifespanText == "" {
				m.LifespanText = trimText(textOf(n))
			}
		}
		return true
	})
}

func parseManagerCell(td *html.Node, m *Match) {
	walk(td, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		m.ManagerProfileURL = attr(n, "href")
		m.ManagerName = trimText(textOf(n))
		return false
	})
}

func parseButtonsCell(td *html.Node, m *Match) {
	walk(td, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		if !hasClass(n, "match-button") {
			return true
		}
		count, _ := strconv.Atoi(attr(n, "data-count"))
		switch {
		case hasClass(n, "tree-match"):
			m.TreeMatchCount = count
		case hasClass(n, "record-match"):
			m.RecordMatchCount = count
		case hasClass(n, "smart-match"):
			m.SmartMatchCount = count
			m.SmartMatchValue, _ = strconv.Atoi(attr(n, "data-value"))
		}
		return true
	})
}

// paginationHasNext returns true if the pagination block contains a
// link to page currentPage+1.
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

// trimText collapses runs of whitespace in s and trims leading/trailing
// space. Geni's HTML has heavy indent whitespace that needs squeezing.
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
// subtree, which carries a localized header label ("Имя:", "Менеджер:")
// that is not part of the value.
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
