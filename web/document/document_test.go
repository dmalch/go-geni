package document_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/document"
)

func newClient(t *testing.T, srv *httptest.Server, csrfPath string) *document.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:        web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:        srv.URL,
		CSRFSourcePath: csrfPath,
		RateLimit:      1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return document.NewClient(wc)
}

func TestGetText_FetchesViewAndScrapesTextarea(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, `<form>
			<input type="hidden" name="authenticity_token" value="csrf-x">
			<textarea name="document[content]">Метрическая книга</textarea>
		</form>`)
	}))
	defer srv.Close()

	text, err := newClient(t, srv, "").GetText(context.Background(), "6000000222741066971")

	Expect(err).ToNot(HaveOccurred())
	Expect(text).To(Equal("Метрическая книга"))
	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/documents/view"))
	Expect(captured.URL.Query().Get("doc_id")).To(Equal("6000000222741066971"))
}

func TestGetText_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()

	_, err := newClient(t, srv, "").GetText(context.Background(), "guid")
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestSaveText_PostsFormAndIncludesCSRF(t *testing.T) {
	RegisterTestingT(t)
	type call struct {
		method string
		path   string
		form   url.Values
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		calls = append(calls, call{method: r.Method, path: r.URL.Path, form: r.Form})
		// First GET (CSRF source) — return a form with a token.
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="tok-1"></form>`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `<html>ok</html>`)
	}))
	defer srv.Close()

	err := newClient(t, srv, "/csrf-source").SaveText(context.Background(), "guid-X", "new body")

	Expect(err).ToNot(HaveOccurred())
	Expect(calls).To(HaveLen(2))

	Expect(calls[0].method).To(Equal(http.MethodGet))
	Expect(calls[0].path).To(Equal("/csrf-source"))

	Expect(calls[1].method).To(Equal(http.MethodPost))
	Expect(calls[1].path).To(Equal("/documents/save_document_content"))
	Expect(calls[1].form.Get("authenticity_token")).To(Equal("tok-1"))
	Expect(calls[1].form.Get("document[id]")).To(Equal("guid-X"))
	Expect(calls[1].form.Get("document[content]")).To(Equal("new body"))
}

func TestSaveText_RefreshesCSRFOn422AndRetries(t *testing.T) {
	RegisterTestingT(t)
	tokens := []string{"stale", "fresh"}
	var postedTokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tok := tokens[0]
			tokens = tokens[1:]
			_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="`+tok+`"></form>`)
			return
		}
		_ = r.ParseForm()
		postedTokens = append(postedTokens, r.Form.Get("authenticity_token"))
		if len(postedTokens) == 1 {
			http.Error(w, "csrf bad", http.StatusUnprocessableEntity)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	err := newClient(t, srv, "/csrf-source").SaveText(context.Background(), "guid", "body")

	Expect(err).ToNot(HaveOccurred())
	Expect(postedTokens).To(Equal([]string{"stale", "fresh"}))
}

func TestSaveText_NonOkResponseReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="tok"></form>`)
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := newClient(t, srv, "/csrf-source").SaveText(context.Background(), "guid", "body")
	Expect(err).To(HaveOccurred())
}
