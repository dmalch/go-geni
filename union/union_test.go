package union

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/profile"
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

func TestGet_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"union-9","status":"spouse"}`)

	u, err := c.Get(context.Background(), "union-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(u.ID).To(Equal("union-9"))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9"))
}

// Geni's bulk-by-id endpoint returns empty results when ids= carries
// exactly one identifier. The client falls back to the singular Get
// for that case; this test asserts the fallback wires the right URL
// and returns the wrapped bulk envelope. Two-id calls still hit the
// bulk path.
func TestGetBulk_SingleIdFallback(t *testing.T) {
	t.Run("single id call goes through singular Get path", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"union-9","status":"spouse"}`)

		res, err := c.GetBulk(context.Background(), []string{"union-9"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].ID).To(Equal("union-9"))
	})

	t.Run("two-id call still hits the bulk endpoint", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"union-9"},{"id":"union-10"}]}`)

		_, err := c.GetBulk(context.Background(), []string{"union-9", "union-10"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("union-9,union-10"))
	})
}

func TestAddPartner_Request(t *testing.T) {
	t.Run("POSTs to /api/<unionId>/add-partner", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-200"}`)

		_, err := c.AddPartner(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9/add-partner"))
	})

	t.Run("decodes the new partner profile", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"id":"profile-200","first_name":"NewPartner","is_alive":true,"public":true}`
		c, _ := newFakeClient(http.StatusOK, body)

		partner, err := c.AddPartner(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(partner.ID).To(Equal("profile-200"))
		Expect(partner.FirstName).ToNot(BeNil())
		Expect(*partner.FirstName).To(Equal("NewPartner"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.AddPartner(context.Background(), "union-9")

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.AddPartner(context.Background(), "union-9")

		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}

func TestAddChild_Request(t *testing.T) {
	t.Run("POSTs to /api/<unionId>/add-child without modifier by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChild(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9/add-child"))
		Expect(ft.lastRequest.URL.Query().Has("relationship_modifier")).To(BeFalse())
	})

	t.Run("WithModifier sets the relationship_modifier query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChild(context.Background(), "union-9", profile.WithModifier("adopt"))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
	})

	t.Run("decodes the new child profile", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"id":"profile-201","first_name":"NewChild","is_alive":true,"public":true}`
		c, _ := newFakeClient(http.StatusOK, body)

		child, err := c.AddChild(context.Background(), "union-9", profile.WithModifier("foster"))

		Expect(err).ToNot(HaveOccurred())
		Expect(child.ID).To(Equal("profile-201"))
		Expect(child.FirstName).ToNot(BeNil())
		Expect(*child.FirstName).To(Equal("NewChild"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.AddChild(context.Background(), "union-9")

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestGetBulk_ThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"union-1","status":"spouse"},
		{"id":"union-2","status":"spouse"},
		{"id":"union-3","status":"ex_spouse"}
	]}`)

	res, err := c.GetBulk(context.Background(), []string{"union-1", "union-2", "union-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("union-1,union-2,union-3"))
	Expect(res.Results).To(HaveLen(3))
	Expect(res.Results[2].Status).To(Equal("ex_spouse"))
}

func TestGetBulk_ErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.GetBulk(context.Background(), []string{"union-1", "union-2"})
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.GetBulk(context.Background(), []string{"union-1", "union-2"})
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}
