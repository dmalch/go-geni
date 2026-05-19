package transport

import (
	"fmt"
	"strings"
)

// EscapeStringToUTF replaces every non-ASCII rune with its \uXXXX
// escape sequence. Geni's API has historically mishandled raw UTF-8
// in request bodies; mutation endpoints route their JSON-encoded body
// through this function before sending. Plain-ASCII input is
// returned unchanged.
func EscapeStringToUTF(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r > 127 {
			fmt.Fprintf(&sb, "\\u%04x", r)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
