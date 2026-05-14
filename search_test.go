package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSearchProfiles_Request(t *testing.T) {
	t.Run("targets /api/profile/search", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.SearchProfiles(context.Background(), "Smith", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile/search"))
	})

	t.Run("names query param is forwarded", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.SearchProfiles(context.Background(), "Jane Doe", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("names")).To(Equal("Jane Doe"))
	})

	t.Run("empty names omits the query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.SearchProfiles(context.Background(), "", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Has("names")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.SearchProfiles(context.Background(), "Smith", 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("zero or negative page omits the param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.SearchProfiles(context.Background(), "Smith", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.SearchProfiles(context.Background(), "Smith", 0)

		Expect(err).To(MatchError(ErrResourceNotFound))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.SearchProfiles(context.Background(), "Smith", 0)

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestSearchProfiles_DecodesResults(t *testing.T) {
	RegisterTestingT(t)
	body := `{
		"results": [
			{"id":"profile-1","first_name":"Jane","last_name":"Doe"},
			{"id":"profile-2","first_name":"John","last_name":"Doe"}
		],
		"page": 1,
		"next_page": "https://www.geni.com/api/profile/search?names=Doe&page=2",
		"prev_page": ""
	}`
	c, _ := newFakeClient(http.StatusOK, body)

	res, err := c.SearchProfiles(context.Background(), "Doe", 1)

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Results).To(HaveLen(2))
	Expect(res.Results[0].Id).To(Equal("profile-1"))
	Expect(res.Results[1].Id).To(Equal("profile-2"))
	Expect(res.Page).To(Equal(1))
	Expect(res.NextPage).To(Equal("https://www.geni.com/api/profile/search?names=Doe&page=2"))
}
