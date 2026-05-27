// Package htmlparse contains small HTML extraction helpers used by the
// AJAX Web client. It is internal — its surface is not part of the
// public API.
package htmlparse

import (
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

var (
	// ErrTokenNotFound is returned when no <input name="authenticity_token">
	// is present in the HTML.
	ErrTokenNotFound = errors.New("authenticity_token not found")
	// ErrTextareaNotFound is returned when no <textarea name="…"> matching
	// the requested name is present in the HTML.
	ErrTextareaNotFound = errors.New("textarea not found")
)

// AuthenticityToken returns the value of the first
// <input name="authenticity_token" value="…"> in r.
func AuthenticityToken(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}
	var token string
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "input" {
			return true
		}
		if attr(n, "name") != "authenticity_token" {
			return true
		}
		token = attr(n, "value")
		return false
	})
	if token == "" {
		return "", ErrTokenNotFound
	}
	return token, nil
}

// TextareaContent returns the text content of the first
// <textarea name="…"> matching name. HTML entities inside the
// textarea are decoded.
func TextareaContent(r io.Reader, name string) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}
	var (
		content string
		found   bool
	)
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "textarea" {
			return true
		}
		if attr(n, "name") != name {
			return true
		}
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				b.WriteString(c.Data)
			}
		}
		content = b.String()
		found = true
		return false
	})
	if !found {
		return "", ErrTextareaNotFound
	}
	return content, nil
}

// RevisionIDs returns every non-empty value of the rev_id attribute
// found in r, in document order.
func RevisionIDs(r io.Reader) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		if v := attr(n, "rev_id"); v != "" {
			ids = append(ids, v)
		}
		return true
	})
	return ids, nil
}

// walk does an in-order DFS. visit returns false to stop the walk.
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
