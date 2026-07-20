package treeconflicts_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/treeconflicts"
)

func newClient(t *testing.T, srv *httptest.Server) *treeconflicts.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return treeconflicts.NewClient(wc)
}

func TestList_ParsesFixtureRows(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/list_tree_conflicts.html")
	Expect(err).ToNot(HaveOccurred())

	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(), treeconflicts.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res).ToNot(BeNil())

	// Request shape: no query params on the default page.
	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/list/tree_conflicts"))
	Expect(captured.URL.RawQuery).To(BeEmpty())

	Expect(res.Page).To(Equal(1))
	Expect(res.HasNext).To(BeTrue()) // fixture is page 1 of 4
	Expect(res.Conflicts).To(HaveLen(3))

	first := res.Conflicts[0]
	Expect(first.ProfileID).To(Equal("6000000218702606843"))
	Expect(first.Name).To(Equal("Акилина Григорьевна Бармина"))
	Expect(first.UpdatedByName).To(Equal("Дмитрий Викторович Мальчиков"))
	Expect(first.ManagerName).To(Equal("Дмитрий Викторович Мальчиков"))
	Expect(first.UpdatedAtText).To(Equal("20.5.2026"))
	Expect(first.TreeURL).To(Equal("/family-tree/index/6000000218702606843?resolve=6000000218702606843"))
	Expect(first.ProfileURL).To(HavePrefix("/people/"))

	// Every row carries an id and an "Open tree" URL keyed to it — the
	// authoritative data.
	for _, c := range res.Conflicts {
		Expect(c.ProfileID).ToNot(BeEmpty())
		Expect(c.TreeURL).To(Equal("/family-tree/index/" + c.ProfileID + "?resolve=" + c.ProfileID))
	}
}

func TestList_QueryParamEncoding(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, `<html><body><table><tbody></tbody></table></body></html>`)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(),
		treeconflicts.ListOptions{Collection: "managed", Page: 3})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Conflicts).To(BeEmpty())
	Expect(res.Page).To(Equal(3))
	Expect(captured.URL.Query().Get("page")).To(Equal("3"))
	Expect(captured.URL.Query().Get("collection")).To(Equal("managed"))
}

func TestList_HasNextDetectionLastPage(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body>
			<table><tbody></tbody></table>
			<ul class="pagination">
				<li><a href="/list/tree_conflicts?page=3">3</a></li>
				<li>4</li>
			</ul>
		</body></html>`)
	}))
	defer srv.Close()
	// Current page 4 has no page=5 link → no next page.
	res, err := newClient(t, srv).List(context.Background(), treeconflicts.ListOptions{Page: 4})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.HasNext).To(BeFalse())
}

func TestList_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).List(context.Background(), treeconflicts.ListOptions{})
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestList_NonOkStatusReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).List(context.Background(), treeconflicts.ListOptions{})
	Expect(err).To(HaveOccurred())
}
