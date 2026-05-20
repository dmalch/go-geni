package user

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestGet_Request(t *testing.T) {
	t.Run("GETs /api/user", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"user-42","name":"Test","account_type":"basic"}`)

		u, err := c.Get(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(u.ID).To(Equal("user-42"))
		Expect(u.Name).To(Equal("Test"))
		Expect(u.AccountType).To(Equal("basic"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user"))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.Get(context.Background())

		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}

func TestFollowedProfiles_Request(t *testing.T) {
	t.Run("GETs /api/user/followed-profiles and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.FollowedProfiles(context.Background(), 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-profiles"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.FollowedProfiles(context.Background(), 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("decodes profile results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.FollowedProfiles(context.Background(), 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestFollowedDocuments_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"document-1","title":"T"}],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.FollowedDocuments(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-documents"))
	Expect(res.Results).To(HaveLen(1))
}

func TestFollowedProjects_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"project-1","name":"P"}]}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.FollowedProjects(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-projects"))
	Expect(res.Results).To(HaveLen(1))
}

func TestFollowedSurnames_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"surname-1","slugged_name":"smith"}]}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.FollowedSurnames(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-surnames"))
	Expect(res.Results).To(HaveLen(1))
	Expect(res.Results[0].SluggedName).To(Equal("smith"))
}

func TestMaxFamily_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"}],"page":1}`)

	res, err := c.MaxFamily(context.Background(), 2)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/max-family"))
	Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	Expect(res.Results).To(HaveLen(1))
}

func TestUploadedPhotos_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"photo-1"}]}`)

	res, err := c.UploadedPhotos(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/uploaded-photos"))
	Expect(res.Results).To(HaveLen(1))
}

func TestUploadedVideos_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"video-1"}]}`)

	res, err := c.UploadedVideos(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/uploaded-videos"))
	Expect(res.Results).To(HaveLen(1))
}

func TestAlbums_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"album-1","name":"Vacation"}],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.Albums(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/my-albums"))
	Expect(res.Results).To(HaveLen(1))
	Expect(res.Results[0].Name).To(Equal("Vacation"))
}

func TestLabels_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":["family","work","travel"],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.Labels(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/my-labels"))
	Expect(res.Results).To(ConsistOf("family", "work", "travel"))
}

func TestMetadata_Request(t *testing.T) {
	t.Run("self call has no ids= param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"data":{"theme":"dark"}}`)

		md, err := c.Metadata(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/metadata"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(string(md.Data)).To(ContainSubstring(`"theme"`))
	})

	t.Run("multi-user call sets ids=", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"data":{}}`)

		_, err := c.Metadata(context.Background(), "user-1", "user-2")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("user-1,user-2"))
	})
}

func TestUpdateMetadata_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"data":{"theme":"light"}}`)

	payload := json.RawMessage(`{"theme":"light"}`)
	md, err := c.UpdateMetadata(context.Background(), payload)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/update-metadata"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"data":"{\"theme\":\"light\"}"`))
	Expect(string(md.Data)).To(ContainSubstring(`"theme"`))
}

func TestUserEndpoints_ErrorMapping(t *testing.T) {
	t.Run("403 → ErrAccessDenied (followed-profiles)", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.FollowedProfiles(context.Background(), 0)
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
	t.Run("404 → ErrResourceNotFound (metadata)", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Metadata(context.Background())
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestManagedProfiles_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"}],"page":1}`)

	res, err := c.ManagedProfiles(context.Background(), 2)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/managed-profiles"))
	Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	Expect(res.Results).To(HaveLen(1))
}
