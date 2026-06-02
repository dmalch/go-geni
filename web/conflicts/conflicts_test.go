package conflicts_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/conflicts"
)

func newClient(t *testing.T, srv *httptest.Server) *conflicts.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return conflicts.NewClient(wc)
}

func TestList_ParsesFixtureRows(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/list_data_conflicts.html")
	Expect(err).ToNot(HaveOccurred())

	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(), conflicts.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res).ToNot(BeNil())

	// Request shape: no page param on the default page.
	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/list/data_conflicts"))
	Expect(captured.URL.Query()).ToNot(HaveKey("page"))

	Expect(res.Page).To(Equal(1))
	Expect(res.HasNext).To(BeFalse()) // single page in the fixture
	Expect(res.Conflicts).To(HaveLen(3))

	first := res.Conflicts[0]
	Expect(first.ProfileGuid).To(Equal("6000000206907048215"))
	Expect(first.Name).To(Equal("Иван Гавриилович Марков"))
	Expect(first.ResolveURL).To(Equal("/merge/resolve/6000000206907048215"))
	Expect(first.ManagerName).To(Equal("Дмитрий Викторович Мальчиков"))
	Expect(first.UpdatedAtText).To(Equal("31.5.2026"))

	// Every row carries a guid and a resolve URL — the authoritative data.
	for _, c := range res.Conflicts {
		Expect(c.ProfileGuid).ToNot(BeEmpty())
		Expect(c.ResolveURL).To(Equal("/merge/resolve/" + c.ProfileGuid))
	}
}

func TestList_PageParamEncoding(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, `<html><body><table><tbody></tbody></table></body></html>`)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(), conflicts.ListOptions{Page: 3})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Conflicts).To(BeEmpty())
	Expect(res.Page).To(Equal(3))
	Expect(captured.URL.Query().Get("page")).To(Equal("3"))
}

func TestList_HasNextDetection(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body>
			<table><tbody></tbody></table>
			<ul class="pagination">
				<li>1</li>
				<li><a href="/list/data_conflicts?page=2">2</a></li>
			</ul>
		</body></html>`)
	}))
	defer srv.Close()
	res, err := newClient(t, srv).List(context.Background(), conflicts.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.HasNext).To(BeTrue())
}

func TestList_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).List(context.Background(), conflicts.ListOptions{})
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestList_NonOkStatusReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).List(context.Background(), conflicts.ListOptions{})
	Expect(err).To(HaveOccurred())
}

func TestGet_ParsesResolveForm(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/resolve_conflict.html")
	Expect(err).ToNot(HaveOccurred())

	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).Get(context.Background(), "6000000206907048215")
	Expect(err).ToNot(HaveOccurred())

	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/merge/resolve/6000000206907048215"))

	Expect(res.ProfileGuid).To(Equal("6000000206907048215"))
	Expect(res.HasConflict).To(BeTrue())

	// One field per hidden resolve[<field>] input.
	bySubject := map[string]conflicts.ConflictField{}
	fields := map[string]bool{}
	for _, f := range res.Fields {
		fields[f.Field] = true
		bySubject[f.Field] = f
	}
	Expect(res.Fields).To(HaveLen(9))
	for _, want := range []string{
		"names/en-US/first_name", "names/ru/first_name",
		"names/en-US/middle_name", "names/ru/middle_name",
		"names/en-US/last_name", "names/ru/last_name",
		"current_residence", "birth_date", "death_date",
	} {
		Expect(fields).To(HaveKey(want))
	}

	// Subject label + primary value are enriched from the table cells.
	ruFirst := bySubject["names/ru/first_name"]
	Expect(ruFirst.Subject).To(Equal("Имя (русский)"))
	Expect(ruFirst.PrimaryValue).To(Equal("Иван"))
}

func TestGet_RedirectToProfileMeansResolved(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/people/Some-Name/6000000206907048215")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).Get(context.Background(), "6000000206907048215")
	Expect(err).ToNot(HaveOccurred())
	Expect(res.HasConflict).To(BeFalse())
	Expect(res.ProfileGuid).To(Equal("6000000206907048215"))
	Expect(res.Fields).To(BeEmpty())
}

func TestResolve_KeepPrimaryPostsPrimaryBlobs(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/resolve_conflict.html")
	Expect(err).ToNot(HaveOccurred())

	type call struct {
		method      string
		requestedBy string
		form        url.Values
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write(fixture)
			return
		}
		_ = r.ParseForm()
		calls = append(calls, call{method: r.Method, requestedBy: r.Header.Get("X-Requested-With"), form: r.Form})
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	cc := newClient(t, srv)

	// The primary value to keep is each field's first data-resolve-data
	// blob, not the "__unchanged__" sentinel (which is a server no-op).
	detail, err := cc.Get(context.Background(), "6000000206907048215")
	Expect(err).ToNot(HaveOccurred())
	primary := map[string]string{}
	for _, f := range detail.Fields {
		Expect(f.DataResolveData).ToNot(BeEmpty())
		primary[f.Field] = f.DataResolveData[0]
	}

	err = cc.Resolve(context.Background(), "6000000206907048215", nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(calls).To(HaveLen(1))

	post := calls[0]
	Expect(post.method).To(Equal(http.MethodPost))
	Expect(post.requestedBy).To(Equal("XMLHttpRequest")) // non-AJAX POST 500s
	// The fixture's CSRF token was scrubbed to TEST_TOKEN.
	Expect(post.form.Get("authenticity_token")).To(Equal("TEST_TOKEN"))
	for _, f := range []string{"names/ru/first_name", "current_residence", "birth_date", "death_date"} {
		Expect(post.form.Get("resolve[" + f + "]")).To(Equal(primary[f]))
		Expect(post.form.Get("resolve[" + f + "]")).ToNot(Equal("__unchanged__"))
		Expect(post.form.Get("resolve[" + f + "]")).ToNot(BeEmpty())
	}
}

func TestResolve_AppliesChoiceForNamedField(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/resolve_conflict.html")
	Expect(err).ToNot(HaveOccurred())

	var postedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write(fixture)
			return
		}
		_ = r.ParseForm()
		postedForm = r.Form
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	err = newClient(t, srv).Resolve(context.Background(), "guid",
		map[string]string{"birth_date": "some-blob"})
	Expect(err).ToNot(HaveOccurred())
	// The named field takes the explicit choice; others default to their
	// primary blob (not "__unchanged__").
	Expect(postedForm.Get("resolve[birth_date]")).To(Equal("some-blob"))
	Expect(postedForm.Get("resolve[death_date]")).ToNot(Equal("__unchanged__"))
	Expect(postedForm.Get("resolve[death_date]")).ToNot(BeEmpty())
}

func TestResolve_AlreadyResolvedIsNoOp(t *testing.T) {
	RegisterTestingT(t)
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Location", "/people/X/guid")
			w.WriteHeader(http.StatusFound)
			return
		}
		posted = true
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	err := newClient(t, srv).Resolve(context.Background(), "guid", nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(posted).To(BeFalse(), "should not POST when already resolved")
}

func TestResolve_RefreshesCSRFOn422AndRetries(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/resolve_conflict.html")
	Expect(err).ToNot(HaveOccurred())

	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write(fixture)
			return
		}
		posts++
		if posts == 1 {
			http.Error(w, "csrf bad", http.StatusUnprocessableEntity)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	err = newClient(t, srv).Resolve(context.Background(), "guid", nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(posts).To(Equal(2))
}

func TestResolve_NonOkResponseReturnsError(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/resolve_conflict.html")
	Expect(err).ToNot(HaveOccurred())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write(fixture)
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err = newClient(t, srv).Resolve(context.Background(), "guid", nil)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("merge/resolve"))
}

func TestResolve_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	err := newClient(t, srv).Resolve(context.Background(), "guid", nil)
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}
