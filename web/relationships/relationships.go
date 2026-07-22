// Package relationships exposes the website-only "change relationship
// modifier" action — flipping a parent↔child edge between biological,
// adopted and foster. The OAuth API can attach a child to a union with a
// modifier but cannot re-tag an existing edge, so this package drives the
// edit_relationships page's parent_modifiers[…] control, POSTing the full
// form back the way the browser's "Save" button does. See the parent
// package doc for legal caveats.
package relationships

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/dmalch/go-geni/web"
)

// Modifiers are the three values Geni's parent_modifiers select accepts.
const (
	ModifierBiological = "bio"
	ModifierAdopted    = "adopt"
	ModifierFoster     = "foster"
)

// validModifiers is the allowed set for a parent relationship modifier.
var validModifiers = map[string]struct{}{
	ModifierBiological: {},
	ModifierAdopted:    {},
	ModifierFoster:     {},
}

// ValidModifier reports whether m is an accepted parent relationship
// modifier (bio/adopt/foster). Callers can use it to reject bad input
// before touching the network.
func ValidModifier(m string) bool {
	_, ok := validModifiers[m]
	return ok
}

// parentModifierRE captures the union web id from a
// parent_modifiers[<id>] form-field name.
var parentModifierRE = regexp.MustCompile(`^parent_modifiers\[([^\]]+)\]$`)

// Client wraps a *web.Client with the relationship-modifier action.
type Client struct {
	web *web.Client
}

// NewClient returns a relationships Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// Result reports the outcome of a SetParentModifier call.
type Result struct {
	// Union is the web union id whose parent relationship was targeted.
	Union string
	// Modifier is the requested modifier (bio/adopt/foster).
	Modifier string
	// Changed is false when the edge already had the requested modifier
	// (no POST was made).
	Changed bool
}

// errCSRFRetry signals a stale authenticity_token (HTTP 422). The caller
// refetches the form (a fresh token) and retries once.
var errCSRFRetry = errors.New("csrf retry needed")

// SetParentModifier changes the relationship modifier of childGUID's edge
// to one of its parent unions. childGUID is the child profile's web guid
// (the 6000000… number). parentUnionWebID selects which parent union to
// re-tag; pass "" to target the child's sole parent union (an error when
// the child has zero or more than one). modifier is bio/adopt/foster.
//
// It mirrors the edit_relationships page's Save: the whole form is
// fetched, the one parent_modifiers[…] field is flipped, and the full
// form is POSTed back, so sibling/spouse/child fields are preserved. When
// the edge already carries modifier the call is a no-op (Changed=false).
// On a 422 (stale token) the form is refetched and the POST retried once.
func (c *Client) SetParentModifier(ctx context.Context, childGUID, parentUnionWebID, modifier string) (Result, error) {
	if _, ok := validModifiers[modifier]; !ok {
		return Result{}, fmt.Errorf("invalid modifier %q: want %s, %s or %s",
			modifier, ModifierBiological, ModifierAdopted, ModifierFoster)
	}
	res, err := c.attempt(ctx, childGUID, parentUnionWebID, modifier)
	if !errors.Is(err, errCSRFRetry) {
		return res, err
	}
	return c.attempt(ctx, childGUID, parentUnionWebID, modifier)
}

func (c *Client) attempt(ctx context.Context, childGUID, parentUnionWebID, modifier string) (Result, error) {
	fields, current, err := c.fetchForm(ctx, childGUID)
	if err != nil {
		return Result{}, err
	}

	target, err := pickUnion(parentUnionWebID, current)
	if err != nil {
		return Result{}, err
	}
	res := Result{Union: target, Modifier: modifier}

	if current[target] == modifier {
		return res, nil // already set — nothing to POST.
	}

	fields.Set("parent_modifiers["+target+"]", modifier)

	u := c.web.BaseURL() + "/profile/edit_relationships/" + url.PathEscape(childGUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(fields.Encode()))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.web.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		_, _ = io.Copy(io.Discard, resp.Body)
		return Result{}, errCSRFRetry
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("edit_relationships: HTTP %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	res.Changed = true
	return res, nil
}

// pickUnion resolves which parent union to target. With an explicit want
// it must be one of the form's parent_modifiers keys; with "" the child
// must have exactly one such key.
func pickUnion(want string, current map[string]string) (string, error) {
	if want != "" {
		if _, ok := current[want]; !ok {
			return "", fmt.Errorf("child has no parent union %q (parent unions: %s)",
				want, strings.Join(sortedKeys(current), ", "))
		}
		return want, nil
	}
	keys := sortedKeys(current)
	switch len(keys) {
	case 0:
		return "", errors.New("child has no parent union to re-tag")
	case 1:
		return keys[0], nil
	default:
		return "", fmt.Errorf("child has %d parent unions (%s); specify which one",
			len(keys), strings.Join(keys, ", "))
	}
}

// fetchForm GETs the child's edit_relationships page and returns every
// #edit_form field as url.Values plus the current parent_modifiers map
// (union web id → selected modifier).
func (c *Client) fetchForm(ctx context.Context, childGUID string) (url.Values, map[string]string, error) {
	u := c.web.BaseURL() + "/profile/edit_relationships/" + url.PathEscape(childGUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.web.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("fetch edit_relationships: HTTP %d", resp.StatusCode)
	}

	root, err := html.Parse(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	form := findByID(root, "edit_form")
	if form == nil {
		return nil, nil, errors.New("edit_relationships: #edit_form not found (session expired?)")
	}

	fields := url.Values{}
	current := map[string]string{}
	walk(form, func(n *html.Node) {
		switch n.Data {
		case "input":
			name := attr(n, "name")
			if name == "" {
				return
			}
			switch attr(n, "type") {
			case "submit", "button", "image", "reset":
				return
			case "checkbox", "radio":
				if !hasAttr(n, "checked") {
					return
				}
			}
			fields.Set(name, attr(n, "value"))
		case "select":
			name := attr(n, "name")
			if name == "" {
				return
			}
			val := selectedOption(n)
			fields.Set(name, val)
			if m := parentModifierRE.FindStringSubmatch(name); m != nil {
				current[m[1]] = val
			}
		case "textarea":
			name := attr(n, "name")
			if name == "" {
				return
			}
			fields.Set(name, textContent(n))
		}
	})
	return fields, current, nil
}

// selectedOption returns the value of the selected <option> under sel, or
// the first option's value when none is marked selected (the browser
// default).
func selectedOption(sel *html.Node) string {
	first := ""
	got := false
	val := ""
	walk(sel, func(n *html.Node) {
		if n.Data != "option" {
			return
		}
		v := attr(n, "value")
		if !got {
			first = v
			got = true
		}
		if hasAttr(n, "selected") {
			val = v
		}
	})
	if val == "" {
		return first
	}
	return val
}

func findByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode && attr(n, "id") == id {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := findByID(c, id); r != nil {
			return r
		}
	}
	return nil
}

func walk(n *html.Node, fn func(*html.Node)) {
	if n.Type == html.ElementNode {
		fn(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
