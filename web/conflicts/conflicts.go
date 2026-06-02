// Package conflicts exposes the website-only "data conflicts" feature —
// the unresolved field disagreements Geni leaves behind after a profile
// merge (conflicting names, dates, residence). The OAuth API has no
// equivalent: GET /api/<id>/{data-conflicts,conflicts,merges} all 500,
// and the merge_pending/merge_note profile fields stay null even when a
// conflict exists. So this package GETs the server-rendered Merge Center
// pages and parses them, mirroring the sibling web/matches package. See
// the parent package doc for legal caveats.
//
// The list is per-user and not reproducible without an account that owns
// merged profiles with pending conflicts, so there is no acceptance test
// for it.
package conflicts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/dmalch/go-geni/web"
)

// resolveKeepPrimary is the sentinel value the resolve form submits for a
// field whose value should stay the surviving (primary) profile's. Every
// hidden resolve[<field>] input defaults to it.
const resolveKeepPrimary = "__unchanged__"

// ListOptions controls the /list/data_conflicts GET parameters. Page is
// 1-based; 0 means "default" (page 1).
type ListOptions struct {
	Page int
}

// Conflict is one row of the data-conflicts list — a merged profile that
// still carries an unresolved field disagreement.
type Conflict struct {
	ProfileGuid   string `json:"profile_guid"`
	Name          string `json:"name,omitempty"`
	ProfileURL    string `json:"profile_url,omitempty"`
	ResolveURL    string `json:"resolve_url"`
	ManagerName   string `json:"manager_name,omitempty"`
	UpdatedAtText string `json:"updated_at_text,omitempty"`
}

// ListResult is one page of conflicts plus pagination state.
type ListResult struct {
	Conflicts []Conflict `json:"conflicts"`
	Page      int        `json:"page"`
	HasNext   bool       `json:"has_next"`
}

// ConflictField is one row of the resolve form's table: a field that
// disagrees between the merged profiles. The authoritative identity is
// Field (from the hidden resolve[<field>] input); Subject/PrimaryValue/
// OtherValues/DataResolveData are best-effort enrichment from the table
// cells. DataResolveData is column-aligned with the displayed values,
// index 0 being the primary (surviving) profile's value.
type ConflictField struct {
	Field           string   `json:"field"`
	Subject         string   `json:"subject,omitempty"`
	PrimaryValue    string   `json:"primary_value,omitempty"`
	OtherValues     []string `json:"other_values,omitempty"`
	DataResolveData []string `json:"-"`
}

// ConflictDetail is the parsed /merge/resolve/<guid> page. When
// HasConflict is false the merge is already resolved (the page 302s to
// the profile) and Fields is empty.
type ConflictDetail struct {
	ProfileGuid string          `json:"profile_guid"`
	HasConflict bool            `json:"has_conflict"`
	Fields      []ConflictField `json:"fields,omitempty"`

	authenticityToken string
}

// Client wraps a *web.Client with the data-conflicts endpoints.
type Client struct {
	web *web.Client
}

// NewClient returns a conflicts Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// List fetches one page of the data-conflicts (Merge Center) list.
func (c *Client) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	u := c.web.BaseURL() + "/list/data_conflicts"
	if opts.Page > 0 {
		u += "?" + url.Values{"page": {fmt.Sprintf("%d", opts.Page)}}.Encode()
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
		return nil, fmt.Errorf("list/data_conflicts: HTTP %d", resp.StatusCode)
	}

	currentPage := opts.Page
	if currentPage <= 0 {
		currentPage = 1
	}
	return parseListConflicts(resp.Body, currentPage)
}

// Get fetches /merge/resolve/<guid> and reports whether the profile still
// has an unresolved data conflict. A resolved profile 302-redirects to
// /people/...; the web client surfaces that without following it (see
// web/client.go), so a redirect away from /merge/resolve means
// HasConflict=false. Otherwise the resolve form is parsed for the
// conflicting fields and the per-form authenticity_token.
func (c *Client) Get(ctx context.Context, guid string) (*ConflictDetail, error) {
	u := c.web.BaseURL() + "/merge/resolve/" + guid
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.web.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// A redirect away from /merge/resolve means the conflict is gone.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc, err := resp.Location(); err != nil || !strings.HasPrefix(loc.Path, "/merge/resolve") {
			return &ConflictDetail{ProfileGuid: guid, HasConflict: false}, nil
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("merge/resolve: HTTP %d", resp.StatusCode)
	}

	detail, err := parseResolveForm(resp.Body)
	if err != nil {
		return nil, err
	}
	detail.ProfileGuid = guid
	return detail, nil
}

// errCSRFRetry signals a stale authenticity_token (HTTP 422). The caller
// refetches the resolve form and retries the POST once.
var errCSRFRetry = errors.New("csrf retry needed")

// Resolve clears a profile's data conflict. choices maps a conflicting
// field name (as returned in ConflictField.Field) to the value to submit;
// any field not in choices keeps the primary (surviving) profile's value.
// Passing a nil/empty map is the "keep primary for every field" default —
// the correct resolution when the survivor is already canonical.
//
// Resolve first re-fetches the resolve form (via Get) to refresh the field
// list and the per-form authenticity_token; if the profile is already
// resolved it returns nil (idempotent). On an HTTP 422 (stale token) it
// refetches once and retries, mirroring matches.Reject.
func (c *Client) Resolve(ctx context.Context, guid string, choices map[string]string) error {
	err := c.postResolve(ctx, guid, choices)
	if err == nil || !errors.Is(err, errCSRFRetry) {
		return err
	}
	return c.postResolve(ctx, guid, choices)
}

func (c *Client) postResolve(ctx context.Context, guid string, choices map[string]string) error {
	detail, err := c.Get(ctx, guid)
	if err != nil {
		return err
	}
	if !detail.HasConflict {
		return nil // already resolved — idempotent.
	}

	// Build the resolution. For each conflicting field, submit the chosen
	// value, defaulting to the primary (surviving) profile's blob — the
	// value the page's "select all → primary" link copies into the input.
	// Submitting resolveKeepPrimary ("__unchanged__") is NOT a resolution:
	// the server treats it as "no selection" and just re-renders the form.
	form := url.Values{"authenticity_token": {detail.authenticityToken}}
	for _, f := range detail.Fields {
		if v, ok := choices[f.Field]; ok {
			form.Set("resolve["+f.Field+"]", v)
			continue
		}
		primary := resolveKeepPrimary
		if len(f.DataResolveData) > 0 && f.DataResolveData[0] != "" {
			primary = f.DataResolveData[0]
		}
		form.Set("resolve["+f.Field+"]", primary)
	}

	u := c.web.BaseURL() + "/merge/resolve/" + guid
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The resolve endpoint 500s on a non-AJAX POST; the page submits it via
	// XMLHttpRequest, so mark the request accordingly.
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.web.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errCSRFRetry
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("merge/resolve: HTTP %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// parseListConflicts walks the data-conflicts HTML page and returns the
// rows plus a HasNext flag.
func parseListConflicts(r io.Reader, currentPage int) (*ListResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	out := &ListResult{Conflicts: []Conflict{}, Page: currentPage}
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "tr":
			if guid := attr(n, "data-profile-id"); guid != "" {
				out.Conflicts = append(out.Conflicts, parseConflictRow(n, guid))
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
// Conflict. The authoritative data is the guid and the /merge/resolve URL;
// name/manager/date are best-effort.
func parseConflictRow(tr *html.Node, guid string) Conflict {
	cf := Conflict{ProfileGuid: guid}
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		switch {
		case hasClass(c, "name-area-grid"):
			cf.Name, cf.ProfileURL = firstLink(c)
		case hasClass(c, "manager-area-grid"):
			cf.ManagerName, _ = firstLink(c)
		case hasClass(c, "update-on-area-grid"):
			cf.UpdatedAtText = trimText(textWithoutAreaTitle(c))
		case hasClass(c, "action-area-grid"):
			cf.ResolveURL = resolveHref(c)
		}
	}
	return cf
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

// resolveHref returns the first /merge/resolve/<guid> link in n.
func resolveHref(n *html.Node) string {
	var found string
	walk(n, func(x *html.Node) bool {
		if found != "" {
			return false
		}
		if x.Type == html.ElementNode && x.Data == "a" {
			if href := attr(x, "href"); strings.HasPrefix(href, "/merge/resolve/") {
				found = href
				return false
			}
		}
		return true
	})
	return found
}

// parseResolveForm walks the /merge/resolve/<guid> page, extracting the
// authenticity_token and one ConflictField per hidden resolve[<field>]
// input. HasConflict is true when at least one such field is present.
func parseResolveForm(r io.Reader) (*ConflictDetail, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	out := &ConflictDetail{Fields: []ConflictField{}}
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "input":
			if out.authenticityToken == "" && attr(n, "name") == "authenticity_token" {
				out.authenticityToken = attr(n, "value")
			}
		case "tr":
			if field, ok := resolveFieldRow(n); ok {
				out.Fields = append(out.Fields, field)
			}
		}
		return true
	})
	out.HasConflict = len(out.Fields) > 0
	return out, nil
}

// resolveFieldRow parses a <tr> of the resolve table into a ConflictField
// if it carries a resolve[<field>] input. The row layout is:
//
//	<td class="subject">LABEL</td>
//	<td> <input name="resolve[FIELD]"> <div data-resolve-data> <a>PRIMARY</a> </td>
//	<td> <div data-resolve-data> <a>OTHER…</a> </td> …
//
// so the input's column is the primary value and later columns are the
// other profiles' values. Void <img> placeholders parse to empty strings.
func resolveFieldRow(tr *html.Node) (ConflictField, bool) {
	var field ConflictField
	var hasInput bool
	var valueCols []struct {
		text string
		blob string
	}

	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "td" {
			continue
		}
		if hasClass(c, "subject") {
			field.Subject = trimText(textWithoutImages(c))
			continue
		}
		// A value column: capture the resolve input name (primary column
		// only), the data-resolve-data blob, and the displayed value.
		if name := resolveInputName(c); name != "" {
			field.Field = strings.TrimSuffix(strings.TrimPrefix(name, "resolve["), "]")
			hasInput = true
		}
		text, _ := firstLink(c)
		valueCols = append(valueCols, struct {
			text string
			blob string
		}{text: text, blob: firstResolveBlob(c)})
	}

	if !hasInput {
		return ConflictField{}, false
	}

	seen := map[string]bool{}
	for i, col := range valueCols {
		field.DataResolveData = append(field.DataResolveData, col.blob)
		if i == 0 {
			field.PrimaryValue = col.text
			seen[col.text] = true
			continue
		}
		// OtherValues highlights real disagreements for human review:
		// drop empty (void) cells and values that merely echo a
		// already-seen one. DataResolveData keeps every column.
		if col.text != "" && !seen[col.text] {
			field.OtherValues = append(field.OtherValues, col.text)
			seen[col.text] = true
		}
	}
	return field, true
}

// resolveInputName returns the name of the first resolve[<field>] input in
// n, or "" if none.
func resolveInputName(n *html.Node) string {
	var name string
	walk(n, func(x *html.Node) bool {
		if name != "" {
			return false
		}
		if x.Type == html.ElementNode && x.Data == "input" {
			if nm := attr(x, "name"); strings.HasPrefix(nm, "resolve[") {
				name = nm
				return false
			}
		}
		return true
	})
	return name
}

// firstResolveBlob returns the submit-ready data-resolve-data value of the
// first element in n that carries one. The attribute is a JSON-quoted,
// URL-encoded ruby-marshal blob ("&quot;%04%08…&quot;"); the page's
// MergeResolver strips the surrounding quotes before placing it in the
// form input, so we do the same — the unquoted blob is exactly what the
// server expects back.
func firstResolveBlob(n *html.Node) string {
	var blob string
	walk(n, func(x *html.Node) bool {
		if blob != "" {
			return false
		}
		if x.Type == html.ElementNode {
			if b := attr(x, "data-resolve-data"); b != "" {
				blob = strings.TrimPrefix(strings.TrimSuffix(b, `"`), `"`)
				return false
			}
		}
		return true
	})
	return blob
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

// textWithoutImages is textOf but skips <img> subtrees, used for the
// subject cell whose label may trail a citation-note icon.
func textWithoutImages(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if x.Type == html.ElementNode && x.Data == "img" {
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
