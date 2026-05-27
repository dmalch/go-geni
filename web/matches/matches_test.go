package matches_test

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
	"github.com/dmalch/go-geni/web/matches"
)

func newClient(t *testing.T, srv *httptest.Server) *matches.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000, // avoid the 1 rps production default in unit tests
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return matches.NewClient(wc)
}

func TestList_ParsesFixtureRows(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/list_matches.html")
	Expect(err).ToNot(HaveOccurred())

	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(), matches.ListOptions{
		Collection: matches.CollectionManaged,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(res).ToNot(BeNil())

	// Request shape.
	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/list/matches"))
	Expect(captured.URL.Query().Get("collection")).To(Equal("managed"))
	Expect(captured.URL.Query()).ToNot(HaveKey("page"))

	// Row count.
	Expect(res.Matches).To(HaveLen(20))
	Expect(res.Page).To(Equal(1))
	Expect(res.HasNext).To(BeTrue())

	// First row fully populated.
	first := res.Matches[0]
	Expect(first.ProfileGuid).To(Equal("6000000225685438084"))
	Expect(first.Name).To(Equal("Иван Гавриилович Котаев"))
	Expect(first.ProfileURL).To(Equal("/people/%D0%98%D0%B2%D0%B0%D0%BD-%D0%9A%D0%BE%D1%82%D0%B0%D0%B5%D0%B2/6000000225685438084"))
	Expect(first.LifespanText).To(Equal("(b. - c.1915)"))
	Expect(first.Deceased).To(BeTrue())
	Expect(first.Privacy).To(Equal("public"))
	Expect(first.Relationship).To(Equal("Дмитрий's second great grandmother's husband"))
	Expect(first.ManagerName).To(Equal("Дмитрий Викторович Мальчиков"))
	Expect(first.ManagerProfileURL).To(Equal("/people/%D0%94%D0%BC%D0%B8%D1%82%D1%80%D0%B8%D0%B9-%D0%9C%D0%B0%D0%BB%D1%8C%D1%87%D0%B8%D0%BA%D0%BE%D0%B2/6000000206907528877"))
	Expect(first.UpdatedAtText).To(Equal("Сегодня"))
	Expect(first.TreeMatchCount).To(Equal(2))
	Expect(first.RecordMatchCount).To(Equal(0))
	Expect(first.SmartMatchCount).To(Equal(1))
	Expect(first.SmartMatchValue).To(Equal(70))

	// Row with empty relationship cell still parses (#4 in the fixture).
	idx := -1
	for i, m := range res.Matches {
		if m.ProfileGuid == "6000000206099980500" {
			idx = i
			break
		}
	}
	Expect(idx).ToNot(Equal(-1), "expected to find row 6000000206099980500")
	row := res.Matches[idx]
	Expect(row.Relationship).To(BeEmpty())
	Expect(row.ManagerName).To(Equal("Anna Polyanicheva"))
	Expect(row.UpdatedAtText).To(Equal("15.5.2026"))
	Expect(row.LifespanText).To(Equal("(1843 - 1850)"))
}

func TestList_QueryParamEncoding(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, `<html><body><table><tbody></tbody></table></body></html>`)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).List(context.Background(), matches.ListOptions{
		Collection: matches.CollectionRelatives,
		Filter:     matches.FilterTreeMatches,
		Order:      matches.OrderUpdatedAt,
		Direction:  matches.DirectionDesc,
		Page:       3,
	})
	Expect(err).ToNot(HaveOccurred())

	q := captured.URL.Query()
	Expect(q.Get("collection")).To(Equal("relatives"))
	Expect(q.Get("filter")).To(Equal("tree_matches"))
	Expect(q.Get("order")).To(Equal("mc_updated_at"))
	Expect(q.Get("direction")).To(Equal("desc"))
	Expect(q.Get("page")).To(Equal("3"))
}

func TestList_ZeroOptionsOmitsParams(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, `<html><body><table><tbody></tbody></table></body></html>`)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).List(context.Background(), matches.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Matches).To(BeEmpty())
	Expect(res.HasNext).To(BeFalse())
	Expect(res.Page).To(Equal(1)) // server's default is page 1

	q := captured.URL.RawQuery
	Expect(q).To(BeEmpty(), "expected no query string, got %q", q)
}

func TestList_HasNextDetection(t *testing.T) {
	t.Run("page=2 link present when on page 1", func(t *testing.T) {
		RegisterTestingT(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `<html><body>
				<table><tbody></tbody></table>
				<ul class="pagination">
					<li>1</li>
					<li><a href="/list/matches?page=2">2</a></li>
				</ul>
			</body></html>`)
		}))
		defer srv.Close()
		res, err := newClient(t, srv).List(context.Background(), matches.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.HasNext).To(BeTrue())
		Expect(res.Page).To(Equal(1))
	})

	t.Run("last page has no next link", func(t *testing.T) {
		RegisterTestingT(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `<html><body>
				<table><tbody></tbody></table>
				<ul class="pagination">
					<li><a href="/list/matches?page=1">1</a></li>
					<li><a href="/list/matches?page=2">2</a></li>
					<li>3</li>
				</ul>
			</body></html>`)
		}))
		defer srv.Close()
		res, err := newClient(t, srv).List(context.Background(), matches.ListOptions{Page: 3})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.HasNext).To(BeFalse())
		Expect(res.Page).To(Equal(3))
	})

	t.Run("no pagination block means single page", func(t *testing.T) {
		RegisterTestingT(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `<html><body><table><tbody></tbody></table></body></html>`)
		}))
		defer srv.Close()
		res, err := newClient(t, srv).List(context.Background(), matches.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.HasNext).To(BeFalse())
	})
}

func TestList_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).List(context.Background(), matches.ListOptions{})
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestList_NonOkStatusReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).List(context.Background(), matches.ListOptions{})
	Expect(err).To(HaveOccurred())
}

func TestForProfile_ParsesFixture(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/search_matches.html")
	Expect(err).ToNot(HaveOccurred())

	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).ForProfile(context.Background(),
		"6000000225685438084", matches.ForProfileOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res).ToNot(BeNil())

	// Request shape: path-style URL, no auto1.
	Expect(captured.Method).To(Equal(http.MethodGet))
	Expect(captured.URL.Path).To(Equal("/search/matches/6000000225685438084"))
	Expect(captured.URL.Query()).ToNot(HaveKey("auto1"))
	Expect(captured.URL.Query()).ToNot(HaveKey("group")) // default group = new, omitted

	// Source profile (the top row).
	Expect(res.Source.ProfileGuid).To(Equal("6000000225685438084"))
	Expect(res.Source.Name).To(Equal("Иван Гавриилович Котаев"))
	Expect(res.Source.ProfileURL).To(Equal("/people/%D0%98%D0%B2%D0%B0%D0%BD-%D0%9A%D0%BE%D1%82%D0%B0%D0%B5%D0%B2/6000000225685438084"))
	Expect(res.Source.LifespanText).To(Equal("(* - >1915)"))
	Expect(res.Source.Deceased).To(BeTrue())
	Expect(res.Source.Privacy).To(Equal("public"))
	Expect(res.Source.PlaceText).To(Equal("село Журавкино, Спасский уезд, Майданская волость, Тамбовская губерния, Россия"))
	Expect(res.Source.ImmediateFamily).To(Equal([]string{
		"Муж Агафьи Тимофеевны Марковой",
		"Отец Валентины Ивановны Котаевой",
	}))
	Expect(res.Source.ManagerName).To(Equal("Дмитрий Викторович Мальчиков"))

	// Matches list — two rows, source row excluded.
	Expect(res.Matches).To(HaveLen(2))
	Expect(res.TotalText).To(ContainSubstring("2"))

	first := res.Matches[0]
	Expect(first.ProfileGuid).To(Equal("6000000225685646845"))
	Expect(first.Name).To(Equal("Иван Гавриилович Котаев"))
	Expect(first.LifespanText).To(Equal("(* - >1911)"))
	Expect(first.Deceased).To(BeTrue())
	Expect(first.Privacy).To(Equal("public"))
	Expect(first.PlaceText).To(Equal("село Журавкино, Спасский уезд, Майданская волость, Тамбовская губерния, Россия"))
	Expect(first.ImmediateFamily).To(Equal([]string{
		"Муж Агафьи Тимофеевны",
		"Отец Никифора Ивановича Котаева",
	}))
	Expect(first.ManagerName).To(Equal("Дмитрий Викторович Мальчиков"))
	Expect(first.SimilarProfilesCount).To(Equal(2))
	Expect(first.CompareURL).To(Equal("/merge/compare/6000000225685438084?return=match%3B&to=6000000225685646845"))

	second := res.Matches[1]
	Expect(second.ProfileGuid).To(Equal("6000000225685564919"))
	Expect(second.LifespanText).To(Equal("(* - >1909)"))
	Expect(second.SimilarProfilesCount).To(Equal(2))
	Expect(second.CompareURL).To(Equal("/merge/compare/6000000225685438084?return=match%3B&to=6000000225685564919"))
}

func TestForProfile_GroupQueryParam(t *testing.T) {
	RegisterTestingT(t)
	for _, c := range []struct {
		group        matches.Group
		expectedPath string
		expectedQS   string
	}{
		{matches.GroupNew, "/search/matches/SRC", ""},
		{matches.GroupRequested, "/search/matches/SRC", "group=requested"},
		{matches.GroupRemoved, "/search/matches/SRC", "group=removed"},
	} {
		var captured *http.Request
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = r.Clone(r.Context())
			_, _ = io.WriteString(w, `<html><body><table></table></body></html>`)
		}))
		_, err := newClient(t, srv).ForProfile(context.Background(), "SRC",
			matches.ForProfileOptions{Group: c.group})
		Expect(err).ToNot(HaveOccurred())
		Expect(captured.URL.Path).To(Equal(c.expectedPath))
		Expect(captured.URL.RawQuery).To(Equal(c.expectedQS))
		Expect(captured.URL.Query()).ToNot(HaveKey("auto1"))
		srv.Close()
	}
}

func TestForProfile_EmptyMatches(t *testing.T) {
	RegisterTestingT(t)
	// Hand-crafted: source row only, no checkbox, no paginator-showing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body><table><tbody>
			<tr class="profile-layout-grid" data-profile-id="111" data-deceased="false" data-privacy="public">
			  <td class="name-grid-area"><span class="strong"><a href="/people/X/111">X</a></span></td>
			  <td class="manager-grid-area"><a href="/people/Y/999">Y</a></td>
			</tr>
		</tbody></table></body></html>`)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).ForProfile(context.Background(), "111", matches.ForProfileOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Source.ProfileGuid).To(Equal("111"))
	Expect(res.Source.Name).To(Equal("X"))
	Expect(res.Matches).To(BeEmpty())
	Expect(res.TotalText).To(BeEmpty())
}

func TestForProfile_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).ForProfile(context.Background(), "111", matches.ForProfileOptions{})
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestForProfile_NonOkStatusReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).ForProfile(context.Background(), "111", matches.ForProfileOptions{})
	Expect(err).To(HaveOccurred())
}
