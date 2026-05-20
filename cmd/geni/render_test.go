package main

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"
)

func TestRender(t *testing.T) {
	t.Run("emits indented JSON with a trailing newline", func(t *testing.T) {
		RegisterTestingT(t)
		var buf bytes.Buffer
		err := render(&buf, map[string]string{"id": "profile-1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(Equal("{\n  \"id\": \"profile-1\"\n}\n"))
	})

	t.Run("does not HTML-escape ampersands or angle brackets", func(t *testing.T) {
		RegisterTestingT(t)
		var buf bytes.Buffer
		err := render(&buf, map[string]string{"note": "A & B <c>"})
		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("A & B <c>"))
	})
}
