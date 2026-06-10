package unions_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/unions"
)

func newClient(t *testing.T, srv *httptest.Server) *unions.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return unions.NewClient(wc)
}

func TestDetach_PostsDeleteRelationshipsWithCSRF(t *testing.T) {
	RegisterTestingT(t)
	type call struct {
		method string
		path   string
		query  url.Values
		form   url.Values
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		calls = append(calls, call{method: r.Method, path: r.URL.Path, query: r.URL.Query(), form: r.Form})
		// First GET (CSRF source) — return a page with a token.
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="tok-1"></form>`)
			return
		}
		_, _ = io.WriteString(w, "<div>ok</div>")
	}))
	defer srv.Close()

	err := newClient(t, srv).Detach(context.Background(),
		"6000000218702418886", []string{"6000000217820816842", "6000000217820816999"})

	Expect(err).ToNot(HaveOccurred())
	Expect(calls).To(HaveLen(2))

	Expect(calls[0].method).To(Equal(http.MethodGet)) // CSRF source fetch

	post := calls[1]
	Expect(post.method).To(Equal(http.MethodPost))
	Expect(post.path).To(Equal("/profile_actions/delete_relationships"))
	Expect(post.query.Get("id")).To(Equal("6000000218702418886"))
	Expect(post.query.Get("uids")).To(Equal("6000000217820816842,6000000217820816999"))
	Expect(post.form.Get("authenticity_token")).To(Equal("tok-1"))
}

func TestDetach_RefreshesCSRFOn422AndRetries(t *testing.T) {
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
		_, _ = io.WriteString(w, "<div>ok</div>")
	}))
	defer srv.Close()

	err := newClient(t, srv).Detach(context.Background(), "pid", []string{"u1"})

	Expect(err).ToNot(HaveOccurred())
	Expect(postedTokens).To(Equal([]string{"stale", "fresh"}))
}

func TestDetach_NonOkResponseReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="tok"></form>`)
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := newClient(t, srv).Detach(context.Background(), "pid", []string{"u1"})
	Expect(err).To(HaveOccurred())
}
