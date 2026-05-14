package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
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
	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	c.client = &http.Client{Transport: ft}
	return c, ft
}

// --- GetImmediateFamily -----------------------------------------------------

func TestGetImmediateFamily_Request(t *testing.T) {
	t.Run("targets /api/<id>/immediate-family", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"focus":{"id":"profile-1"},"nodes":{}}`)

		_, err := c.GetImmediateFamily(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/immediate-family"))
	})

	t.Run("decodes focus and nodes", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"focus":{"id":"profile-1","first_name":"A"},"nodes":{"profile-1":{"id":"profile-1","first_name":"A"},"union-9":{"id":"union-9"}}}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetImmediateFamily(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Focus).ToNot(BeNil())
		Expect(res.Focus.Id).To(Equal("profile-1"))
		Expect(res.Nodes).To(HaveKey("profile-1"))
		Expect(res.Nodes).To(HaveKey("union-9"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.GetImmediateFamily(context.Background(), "profile-1")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.GetImmediateFamily(context.Background(), "profile-1")

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestFamilyNodes_Accessors(t *testing.T) {
	RegisterTestingT(t)
	body := `{
		"focus": {"id":"profile-1"},
		"nodes": {
			"profile-1": {"id":"profile-1","first_name":"A"},
			"profile-2": {"id":"profile-2","first_name":"B"},
			"union-9": {"id":"union-9","partners":["profile-1","profile-2"]}
		}
	}`
	var res FamilyResponse
	Expect(json.Unmarshal([]byte(body), &res)).To(Succeed())

	t.Run("Profile decodes a profile node", func(t *testing.T) {
		RegisterTestingT(t)
		p, err := res.Nodes.Profile("profile-2")
		Expect(err).ToNot(HaveOccurred())
		Expect(p.Id).To(Equal("profile-2"))
		Expect(p.FirstName).ToNot(BeNil())
		Expect(*p.FirstName).To(Equal("B"))
	})

	t.Run("Union decodes a union node", func(t *testing.T) {
		RegisterTestingT(t)
		u, err := res.Nodes.Union("union-9")
		Expect(err).ToNot(HaveOccurred())
		Expect(u.Id).To(Equal("union-9"))
		Expect(u.Partners).To(ConsistOf("profile-1", "profile-2"))
	})

	t.Run("Profile rejects non-profile ids", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := res.Nodes.Profile("union-9")
		Expect(err).To(HaveOccurred())
	})

	t.Run("Union rejects non-union ids", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := res.Nodes.Union("profile-1")
		Expect(err).To(HaveOccurred())
	})

	t.Run("ProfileIds returns only profile- keys", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(res.Nodes.ProfileIds()).To(ConsistOf("profile-1", "profile-2"))
	})

	t.Run("UnionIds returns only union- keys", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(res.Nodes.UnionIds()).To(ConsistOf("union-9"))
	})
}

// --- GetAncestors -----------------------------------------------------------

func TestGetAncestors_Request(t *testing.T) {
	t.Run("targets /api/<id>/ancestors and omits generations by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"focus":{"id":"profile-1"},"nodes":{}}`)

		_, err := c.GetAncestors(context.Background(), "profile-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/ancestors"))
		Expect(ft.lastRequest.URL.Query().Has("generations")).To(BeFalse())
	})

	t.Run("WithGenerations sets generations query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"focus":{"id":"profile-1"},"nodes":{}}`)

		_, err := c.GetAncestors(context.Background(), "profile-1", WithGenerations(10))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("generations")).To(Equal("10"))
	})

	t.Run("WithGenerations clamps to 20", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"focus":{"id":"profile-1"},"nodes":{}}`)

		_, err := c.GetAncestors(context.Background(), "profile-1", WithGenerations(25))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("generations")).To(Equal("20"))
	})

	t.Run("WithGenerations is a no-op for zero or negative", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"focus":{"id":"profile-1"},"nodes":{}}`)

		_, err := c.GetAncestors(context.Background(), "profile-1", WithGenerations(0))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Has("generations")).To(BeFalse())
	})
}

// --- GetPathTo --------------------------------------------------------------

func TestGetPathTo_Request(t *testing.T) {
	t.Run("targets /api/<from>/path-to/<to>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"status":"done"}`)

		_, err := c.GetPathTo(context.Background(), "profile-1", "profile-2")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/path-to/profile-2"))
	})

	t.Run("WithPathType sets path_type", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"status":"done"}`)

		_, err := c.GetPathTo(context.Background(), "profile-1", "profile-2",
			WithPathType(PathTypeBlood))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("path_type")).To(Equal("blood"))
	})

	t.Run("bool options emit only when set", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"status":"done"}`)

		_, err := c.GetPathTo(context.Background(), "profile-1", "profile-2",
			WithRefresh(true), WithSkipEmail(true), WithSkipNotify(true))

		Expect(err).ToNot(HaveOccurred())
		q := ft.lastRequest.URL.Query()
		Expect(q.Get("refresh")).To(Equal("true"))
		Expect(q.Get("skip_email")).To(Equal("true"))
		Expect(q.Get("skip_notify")).To(Equal("true"))
		Expect(q.Has("search")).To(BeFalse())
	})

	t.Run("WithSearch(false) emits search=false (Geni default is true)", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"status":"done"}`)

		_, err := c.GetPathTo(context.Background(), "profile-1", "profile-2",
			WithSearch(false))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("search")).To(Equal("false"))
	})

	t.Run("bool options are no-ops at their default polarity", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"status":"done"}`)

		_, err := c.GetPathTo(context.Background(), "profile-1", "profile-2",
			WithRefresh(false), WithSearch(true),
			WithSkipEmail(false), WithSkipNotify(false))

		Expect(err).ToNot(HaveOccurred())
		q := ft.lastRequest.URL.Query()
		Expect(q.Has("refresh")).To(BeFalse())
		Expect(q.Has("search")).To(BeFalse())
		Expect(q.Has("skip_email")).To(BeFalse())
		Expect(q.Has("skip_notify")).To(BeFalse())
	})
}

func TestGetPathTo_DecodesStatus(t *testing.T) {
	cases := map[string]PathStatus{
		`{"status":"pending"}`:    PathStatusPending,
		`{"status":"done"}`:       PathStatusDone,
		`{"status":"overloaded"}`: PathStatusOverloaded,
		`{"status":"not found"}`:  PathStatusNotFound,
	}
	for body, want := range cases {
		t.Run(string(want), func(t *testing.T) {
			RegisterTestingT(t)
			c, _ := newFakeClient(http.StatusOK, body)

			res, err := c.GetPathTo(context.Background(), "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(want))
		})
	}
}
