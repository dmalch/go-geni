package profile

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/transport"
)

type fakeTransport struct {
	lastRequest *http.Request
	status      int
	body        string
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.lastRequest = req.Clone(req.Context())
	body := t.body
	if body == "" {
		body = "{}"
	}
	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}, nil
}

func newFakeClient(status int, body string) (*Client, *fakeTransport) {
	ft := &fakeTransport{status: status, body: body}
	t := transport.New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	t.SetHTTPClient(&http.Client{Transport: ft})
	return NewClient(t), ft
}

func TestCreate_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-9","guid":"g-9"}`)

	first := "Alice"
	p, err := c.Create(context.Background(), &Request{
		Names: map[string]NameElement{"en-US": {FirstName: &first}},
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(p.ID).To(Equal("profile-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile/add"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"first_name":"Alice"`))
}

func TestGet_Request(t *testing.T) {
	t.Run("GETs /api/<id> with the fields param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

		p, err := c.Get(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(p.ID).To(Equal("profile-1"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1"))
		Expect(ft.lastRequest.URL.Query().Has("fields")).To(BeTrue())
	})

	t.Run("strips the API URL prefix from unions", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusOK,
			`{"id":"profile-1","unions":["https://api.sandbox.geni.com/union-7"]}`)

		p, err := c.Get(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(p.Unions).To(ConsistOf("union-7"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Get(context.Background(), "profile-1")
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestGetBulk_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

		res, err := c.GetBulk(context.Background(), []string{"profile-1"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
	})

	t.Run("2 ids → /api/profile?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"},{"id":"profile-2"}]}`)

		_, err := c.GetBulk(context.Background(), []string{"profile-1", "profile-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("profile-1,profile-2"))
	})
}

func TestUpdate_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1","first_name":"After"}`)

	first := "After"
	_, err := c.Update(context.Background(), "profile-1", &Request{
		Names: map[string]NameElement{"en-US": {FirstName: &first}},
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/update"))
}

func TestUpdateBasics_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1","first_name":"After"}`)

	first := "After"
	_, err := c.UpdateBasics(context.Background(), "profile-1", &Request{
		Names: map[string]NameElement{"en-US": {FirstName: &first}},
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/update-basics"))
}

func TestDelete_Request(t *testing.T) {
	t.Run("POSTs to /api/<id>/delete", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

		err := c.Delete(context.Background(), "profile-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-9/delete"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		err := c.Delete(context.Background(), "profile-9")
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestAddPartner_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-200"}`)

	_, err := c.AddPartner(context.Background(), "profile-1")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-partner"))
}

func TestAddChild_Request(t *testing.T) {
	t.Run("WithModifier sets relationship_modifier", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChild(context.Background(), "profile-1", WithModifier("foster"))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-child"))
		Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("foster"))
	})

	t.Run("without options omits the modifier", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChild(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Has("relationship_modifier")).To(BeFalse())
	})
}

func TestAddSibling_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-202"}`)

	_, err := c.AddSibling(context.Background(), "profile-1", WithModifier("adopt"))

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-sibling"))
	Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
}

func TestAddParent_Request(t *testing.T) {
	t.Run("POSTs JSON to /api/<id>/add-parent", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-parent","first_name":"Mom"}`)

		first := "Mom"
		_, err := c.AddParent(context.Background(), "profile-1", &Request{
			Names: map[string]NameElement{"en-US": {FirstName: &first}},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-parent"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"first_name":"Mom"`))
	})

	t.Run("WithModifier sets the relationship_modifier query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-parent"}`)

		_, err := c.AddParent(context.Background(), "profile-1", &Request{}, WithModifier("adopt"))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
	})
}

func TestMerge_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

	err := c.Merge(context.Background(), "profile-1", "profile-2")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/merge/profile-2"))
}

func TestFollow_Request(t *testing.T) {
	t.Run("Follow POSTs to /follow", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

		_, err := c.Follow(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/follow"))
	})

	t.Run("Unfollow POSTs to /unfollow", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

		_, err := c.Unfollow(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/unfollow"))
	})

	t.Run("404 → ErrResourceNotFound, 403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Follow(context.Background(), "profile-1")
		Expect(err).To(MatchError(transport.ErrResourceNotFound))

		c, _ = newFakeClient(http.StatusForbidden, ``)
		_, err = c.Follow(context.Background(), "profile-1")
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}

func TestWipeEventDates_Request(t *testing.T) {
	t.Run("POSTs date-wipe payload to /api/<id>/update", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

		err := c.WipeEventDates(context.Background(), "profile-1", []string{"birth", "death"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/update"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"birth":{"date":{}}`))
		Expect(string(got)).To(ContainSubstring(`"death":{"date":{}}`))
	})

	t.Run("empty eventKeys is a no-op (no request sent)", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		err := c.WipeEventDates(context.Background(), "profile-1", nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest).To(BeNil())
	})
}

func TestGetBulk_ThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"profile-1","first_name":"A"},
		{"id":"profile-2","first_name":"B"},
		{"id":"profile-3","first_name":"C"}
	]}`)

	res, err := c.GetBulk(context.Background(), []string{"profile-1", "profile-2", "profile-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("profile-1,profile-2,profile-3"))
	Expect(res.Results).To(HaveLen(3))
	ids := []string{res.Results[0].ID, res.Results[1].ID, res.Results[2].ID}
	Expect(ids).To(ConsistOf("profile-1", "profile-2", "profile-3"))
}

func TestGetBulk_ErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.GetBulk(context.Background(), []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.GetBulk(context.Background(), []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}
