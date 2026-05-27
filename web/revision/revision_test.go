package revision_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/revision"
)

func newClient(t *testing.T, srv *httptest.Server) *revision.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return revision.NewClient(wc)
}

func TestForProfile_PostsGuidAndParsesIDs(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		body, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `<div id="revisions_container">
			<div rev_id="88793956740">…</div>
			<div rev_id="88793956730">…</div>
		</div>`)
	}))
	defer srv.Close()

	ids, err := newClient(t, srv).ForProfile(context.Background(), "6000000218702371879")

	Expect(err).ToNot(HaveOccurred())
	Expect(ids).To(Equal([]string{"88793956740", "88793956730"}))
	Expect(captured.Method).To(Equal(http.MethodPost))
	Expect(captured.URL.Path).To(Equal("/revisions/profile"))
	Expect(captured.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
	Expect(string(body)).To(Equal("id=6000000218702371879"))
}

func TestForProfile_EmptyContainerReturnsEmpty(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<div id="revisions_container"></div>`)
	}))
	defer srv.Close()

	ids, err := newClient(t, srv).ForProfile(context.Background(), "guid")

	Expect(err).ToNot(HaveOccurred())
	Expect(ids).To(BeEmpty())
}

func TestForProfile_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).ForProfile(context.Background(), "guid")
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestForProfile_NonOkStatusReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).ForProfile(context.Background(), "guid")
	Expect(err).To(HaveOccurred())
}
