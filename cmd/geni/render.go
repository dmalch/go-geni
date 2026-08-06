package main

import (
	"encoding/json"
	"io"
)

// render writes v to w as indented JSON. HTML escaping is disabled so
// genealogy names and URLs read naturally.
func render(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// renderList writes a JSON array, never `null`.
//
// The paginated list commands aggregate into `var out []T`; when the queue is
// empty nothing is appended and the nil slice marshals as `null` — with exit 0
// and no stderr, so a consumer cannot distinguish "no results" from a failure
// without checking the exit code. A list command must always emit a list.
func renderList[T any](w io.Writer, items []T) error {
	if items == nil {
		items = []T{}
	}
	return render(w, items)
}
