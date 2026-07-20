package treeconflicts_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// showServer routes the two (or three) requests Show makes: the tree page
// (treeSessionId source), fetch_immediate_family (the conflict JSON), and
// fetch_prune_counts (subtree sizes). immediateFamily/pruneCounts bodies
// are provided by the caller.
func showServer(t *testing.T, immediateFamily, pruneCounts string, captured *map[string]*http.Request) *httptest.Server {
	t.Helper()
	index, err := os.ReadFile("testdata/tree_index.html")
	if err != nil {
		t.Fatalf("read index fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			(*captured)[r.URL.Path] = r.Clone(r.Context())
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/family-tree/index/"):
			_, _ = w.Write(index)
		case r.URL.Path == "/flash/fetch_immediate_family":
			_, _ = io.WriteString(w, immediateFamily)
		case r.URL.Path == "/flash/fetch_prune_counts":
			_, _ = io.WriteString(w, pruneCounts)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestShow_ParsesConflict(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/fetch_immediate_family.json")
	Expect(err).ToNot(HaveOccurred())

	pruneCounts := `{"prune_counts":[
		{"pid":401078841,"p":"+110"},{"pid":401078391,"p":"+1"},
		{"pid":401211169,"p":"+38"},{"pid":402225494,"p":"+2"}]}`

	captured := map[string]*http.Request{}
	srv := showServer(t, string(fixture), pruneCounts, &captured)
	defer srv.Close()

	d, err := newClient(t, srv).Show(context.Background(), "6000000218702606843")
	Expect(err).ToNot(HaveOccurred())

	// The immediate-family request carries the conflict probes + AJAX header.
	imm := captured["/flash/fetch_immediate_family"]
	Expect(imm).ToNot(BeNil())
	Expect(imm.Header.Get("X-Requested-With")).To(Equal("XMLHttpRequest"))
	Expect(imm.URL.Query().Get("resolve_duplicates")).To(Equal("true"))
	Expect(imm.URL.Query().Get("check_partner_conflicts")).To(Equal("true"))
	Expect(imm.URL.Query().Get("profile")).To(Equal("6000000218702606843"))
	// treeSessionId came from the tree page fixture.
	Expect(imm.URL.Query().Get("treeSessionId")).To(Equal(
		"d9f2cbaf3850c4c3ae098d777dfdefa277822c11cbecc60002041238477a0c5b"))

	Expect(d.HasConflict).To(BeTrue())
	Expect(d.ConflictTypes).To(Equal([]string{"duplicate_parents"}))
	Expect(d.ParentUnionCount).To(Equal(2))
	Expect(d.PartnerConflict).To(BeTrue())
	Expect(d.Focus.ProfileID).To(Equal("6000000218702606843"))
	Expect(d.Focus.Name).To(Equal("Акилина Григорьевна Бармина"))
	Expect(d.ParentUnions).To(HaveLen(2))

	// A father duplicate-candidate holding the two "Григорий" profiles.
	var father *treeconflicts.DuplicateCandidate
	for i := range d.DuplicateCandidates {
		if d.DuplicateCandidates[i].Role == "father" {
			father = &d.DuplicateCandidates[i]
		}
	}
	Expect(father).ToNot(BeNil())
	Expect(father.Profiles).To(HaveLen(2))
	guids := []string{father.Profiles[0].ProfileID, father.Profiles[1].ProfileID}
	Expect(guids).To(ConsistOf("6000000217733708841", "6000000217733583911"))
	// Subtree sizes were merged in from fetch_prune_counts.
	for _, p := range father.Profiles {
		Expect(p.SubtreeSize).ToNot(BeEmpty())
	}
}

func TestShow_SuggestedActions(t *testing.T) {
	RegisterTestingT(t)
	fixture, err := os.ReadFile("testdata/fetch_immediate_family.json")
	Expect(err).ToNot(HaveOccurred())

	// Father 708841 has the larger subtree (+110) than 583911 (+1), so it is
	// kept and the suggestion merges the smaller into it.
	pruneCounts := `{"prune_counts":[
		{"pid":401078841,"p":"+110"},{"pid":401078391,"p":"+1"},
		{"pid":401211169,"p":"+38"},{"pid":402225494,"p":"+2"}]}`

	srv := showServer(t, string(fixture), pruneCounts, nil)
	defer srv.Close()

	d, err := newClient(t, srv).Show(context.Background(), "6000000218702606843")
	Expect(err).ToNot(HaveOccurred())

	Expect(d.SuggestedActions).To(ContainElement(
		"geni profile compare profile-g6000000217733708841 profile-g6000000217733583911"))
	Expect(d.SuggestedActions).To(ContainElement(HavePrefix(
		"geni profile merge profile-g6000000217733708841 profile-g6000000217733583911")))
}

func TestShow_NoConflict(t *testing.T) {
	RegisterTestingT(t)
	// A focus with a single parent union → no conflict.
	single := `{"tree":{
		"unions":[{"u":1,"p":[10,11],"c":[20]}],
		"nodes":[
			{"n":20,"pid":"p20","pr_id":"g20","g":"f","nm":"Child","focus":1,"npu":"1"},
			{"n":10,"pid":"p10","pr_id":"g10","g":"m","nm":"Father"},
			{"n":11,"pid":"p11","pr_id":"g11","g":"f","nm":"Mother"}]}}`
	srv := showServer(t, single, "", nil)
	defer srv.Close()

	d, err := newClient(t, srv).Show(context.Background(), "g20")
	Expect(err).ToNot(HaveOccurred())
	Expect(d.HasConflict).To(BeFalse())
	Expect(d.ConflictTypes).To(BeEmpty())
	Expect(d.DuplicateCandidates).To(BeEmpty())
	Expect(d.SuggestedActions).To(BeEmpty())
}

func TestShow_LoginRedirectMapsToErrNotLoggedIn(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	_, err := newClient(t, srv).Show(context.Background(), "6000000218702606843")
	Expect(errors.Is(err, web.ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}
