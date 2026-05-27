package htmlparse

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestAuthenticityToken(t *testing.T) {
	t.Run("extracts authenticity_token from a form input", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<html><body>
			<form action="/documents/save_document_content" method="post">
				<input type="hidden" name="authenticity_token" value="knhS0rpb5T7WaJt1+bWc==">
				<textarea name="document[content]">hi</textarea>
			</form>
		</body></html>`

		tok, err := AuthenticityToken(strings.NewReader(html))

		Expect(err).ToNot(HaveOccurred())
		Expect(tok).To(Equal("knhS0rpb5T7WaJt1+bWc=="))
	})

	t.Run("returns ErrTokenNotFound when no input present", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := AuthenticityToken(strings.NewReader(`<html><body></body></html>`))

		Expect(err).To(MatchError(ErrTokenNotFound))
	})

	t.Run("ignores non-authenticity_token inputs", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<form>
			<input type="text" name="other" value="nope">
			<input type="hidden" name="authenticity_token" value="real-token">
		</form>`

		tok, err := AuthenticityToken(strings.NewReader(html))

		Expect(err).ToNot(HaveOccurred())
		Expect(tok).To(Equal("real-token"))
	})
}

func TestTextareaContent(t *testing.T) {
	t.Run("extracts named textarea content verbatim", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<form>
			<textarea name="document[content]">Метрическая книга Авдалово 1784
Архив: ГАРО (Рязань)</textarea>
		</form>`

		body, err := TextareaContent(strings.NewReader(html), "document[content]")

		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(Equal("Метрическая книга Авдалово 1784\nАрхив: ГАРО (Рязань)"))
	})

	t.Run("decodes HTML entities in textarea content", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<textarea name="document[content]">a &lt; b &amp;&amp; c &gt; d</textarea>`

		body, err := TextareaContent(strings.NewReader(html), "document[content]")

		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(Equal("a < b && c > d"))
	})

	t.Run("returns ErrTextareaNotFound when textarea absent", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := TextareaContent(strings.NewReader(`<form></form>`), "document[content]")

		Expect(err).To(MatchError(ErrTextareaNotFound))
	})

	t.Run("ignores textareas with a different name", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<form><textarea name="other">nope</textarea></form>`

		_, err := TextareaContent(strings.NewReader(html), "document[content]")

		Expect(err).To(MatchError(ErrTextareaNotFound))
	})
}

func TestRevisionIDs(t *testing.T) {
	t.Run("extracts rev_id attribute values in document order", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<div id="revisions_container">
			<div rev_id="88793956740">…</div>
			<div rev_id="88793956730">…</div>
			<div rev_id="88793955900">…</div>
		</div>`

		ids, err := RevisionIDs(strings.NewReader(html))

		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(Equal([]string{"88793956740", "88793956730", "88793955900"}))
	})

	t.Run("returns empty slice when no rev_id attributes present", func(t *testing.T) {
		RegisterTestingT(t)

		ids, err := RevisionIDs(strings.NewReader(`<html><body><div>nothing</div></body></html>`))

		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(BeEmpty())
	})

	t.Run("ignores empty rev_id values", func(t *testing.T) {
		RegisterTestingT(t)
		html := `<div rev_id="">skip</div><div rev_id="42">keep</div>`

		ids, err := RevisionIDs(strings.NewReader(html))

		Expect(err).ToNot(HaveOccurred())
		Expect(ids).To(Equal([]string{"42"}))
	})
}
