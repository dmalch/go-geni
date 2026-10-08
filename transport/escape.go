package transport

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// EscapeStringToUTF replaces every non-ASCII rune with its \uXXXX
// escape sequence. Geni's API has historically mishandled raw UTF-8
// in request bodies; mutation endpoints route their JSON-encoded body
// through this function before sending. Plain-ASCII input is
// returned unchanged.
//
// A JSON escape holds one UTF-16 code unit, so a rune outside the Basic
// Multilingual Plane — an emoji, say — is written as a surrogate pair.
func EscapeStringToUTF(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r <= 127:
			sb.WriteRune(r)
		case r > 0xFFFF:
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&sb, "\\u%04x\\u%04x", hi, lo)
		default:
			fmt.Fprintf(&sb, "\\u%04x", r)
		}
	}
	return sb.String()
}
