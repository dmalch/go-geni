package relationships_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/relationships"
)

const testUnionID = "600123"

// editForm renders a minimal edit_relationships page whose #edit_form
// carries one parent union with the given modifier selected. Mirrors the
// real page's field set (authenticity_token, valid_union_ids,
// parent_unions[…], parent_modifiers[…], redirect_tab).
func editForm(token, modifier string) string {
	unionID := testUnionID
	opt := func(v, label string) string {
		sel := ""
		if v == modifier {
			sel = ` selected="selected"`
		}
		return `<option value="` + v + `"` + sel + `>` + label + `</option>`
	}
	return `<html><body>
<form action="/profile/edit_relationships/CHILD" id="edit_form" method="post">
<input name="authenticity_token" type="hidden" value="` + token + `"/>
<input name="valid_union_ids" type="hidden" value="[&quot;` + unionID + `&quot;]"/>
<select name="parent_unions[` + unionID + `]"><option value="` + unionID + `" selected="selected">Parents</option></select>
<select class="parent-modifier-selector" name="parent_modifiers[` + unionID + `]">` +
		opt("bio", "Биологические") + opt("adopt", "Приёмные") + opt("foster", "Патронатные") +
		`</select>
<input name="redirect_tab" type="hidden" value=""/>
</form></body></html>`
}

func newClient(t *testing.T, srv *httptest.Server) *relationships.Client {
	t.Helper()
	wc, err := web.NewClient(web.Options{
		Cookies:   web.CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 1000,
	})
	if err != nil {
		t.Fatalf("web.NewClient: %v", err)
	}
	return relationships.NewClient(wc)
}

type call struct {
	method string
	path   string
	form   url.Values
}

func TestSetParentModifier_FlipsBioToFoster(t *testing.T) {
	RegisterTestingT(t)
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		calls = append(calls, call{method: r.Method, path: r.URL.Path, form: r.Form})
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, editForm("tok-1", "bio"))
			return
		}
		_, _ = io.WriteString(w, "<div>ok</div>")
	}))
	defer srv.Close()

	res, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "", "foster")

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Union).To(Equal("600123"))
	Expect(res.Modifier).To(Equal("foster"))
	Expect(res.Changed).To(BeTrue())

	Expect(calls).To(HaveLen(2))
	Expect(calls[0].method).To(Equal(http.MethodGet))
	Expect(calls[0].path).To(Equal("/profile/edit_relationships/CHILD"))

	post := calls[1]
	Expect(post.method).To(Equal(http.MethodPost))
	Expect(post.path).To(Equal("/profile/edit_relationships/CHILD"))
	// The one flipped field.
	Expect(post.form.Get("parent_modifiers[600123]")).To(Equal("foster"))
	// Every other field preserved verbatim.
	Expect(post.form.Get("authenticity_token")).To(Equal("tok-1"))
	Expect(post.form.Get("valid_union_ids")).To(Equal(`["600123"]`))
	Expect(post.form.Get("parent_unions[600123]")).To(Equal("600123"))
	Expect(post.form).To(HaveKey("redirect_tab"))
}

func TestSetParentModifier_NoOpWhenAlreadySet(t *testing.T) {
	RegisterTestingT(t)
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
		}
		_, _ = io.WriteString(w, editForm("tok-1", "foster"))
	}))
	defer srv.Close()

	res, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "", "foster")

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Changed).To(BeFalse())
	Expect(res.Union).To(Equal("600123"))
	Expect(posts).To(Equal(0), "no POST when the modifier already matches")
}

func TestSetParentModifier_InvalidModifierRejectedBeforeHTTP(t *testing.T) {
	RegisterTestingT(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, editForm("t", "bio"))
	}))
	defer srv.Close()

	_, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "", "sibling")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("modifier"))
	Expect(hits).To(Equal(0), "invalid modifier must not touch the network")
}

func TestSetParentModifier_AmbiguousParentUnionErrors(t *testing.T) {
	RegisterTestingT(t)
	page := `<form id="edit_form" action="/profile/edit_relationships/CHILD" method="post">
<input name="authenticity_token" value="t"/>
<select name="parent_modifiers[A]"><option value="bio" selected>b</option><option value="foster">f</option></select>
<select name="parent_modifiers[B]"><option value="bio" selected>b</option><option value="foster">f</option></select>
</form>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	_, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "", "foster")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("parent union"))
}

func TestSetParentModifier_ExplicitParentUnionPicksKey(t *testing.T) {
	RegisterTestingT(t)
	page := `<form id="edit_form" action="/profile/edit_relationships/CHILD" method="post">
<input name="authenticity_token" value="t"/>
<select name="parent_modifiers[A]"><option value="bio" selected>b</option><option value="foster">f</option></select>
<select name="parent_modifiers[B]"><option value="bio" selected>b</option><option value="foster">f</option></select>
</form>`
	var post call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Method == http.MethodPost {
			post = call{form: r.Form}
		}
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	res, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "B", "foster")
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Union).To(Equal("B"))
	Expect(post.form.Get("parent_modifiers[B]")).To(Equal("foster"))
	Expect(post.form.Get("parent_modifiers[A]")).To(Equal("bio"), "the other union is left untouched")
}

func TestSetParentModifier_UnknownParentUnionErrors(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, editForm("t", "bio"))
	}))
	defer srv.Close()

	_, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "999", "foster")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("999"))
}

func TestSetParentModifier_RetriesOn422(t *testing.T) {
	RegisterTestingT(t)
	var gets, posts int
	var postedTokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Method == http.MethodGet {
			gets++
			// Fresh token each GET so the retry can be observed.
			_, _ = io.WriteString(w, editForm("tok-"+strings.Repeat("x", gets), "bio"))
			return
		}
		posts++
		postedTokens = append(postedTokens, r.Form.Get("authenticity_token"))
		if posts == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		_, _ = io.WriteString(w, "<div>ok</div>")
	}))
	defer srv.Close()

	res, err := newClient(t, srv).SetParentModifier(context.Background(), "CHILD", "", "foster")
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Changed).To(BeTrue())
	Expect(gets).To(Equal(2))
	Expect(posts).To(Equal(2))
	Expect(postedTokens[0]).ToNot(Equal(postedTokens[1]), "retry must use a freshly fetched token")
}
