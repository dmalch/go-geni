package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetSurname_Request(t *testing.T) {
	t.Run("GETs /api/<surnameId>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"surname-1","slugged_name":"smith","description":"Smith family"}`)

		s, err := c.GetSurname(context.Background(), "surname-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(s.Id).To(Equal("surname-1"))
		Expect(s.SluggedName).To(Equal("smith"))
		Expect(s.Description).To(Equal("Smith family"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.GetSurname(context.Background(), "surname-1")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}

func TestGetSurnameFollowers_Request(t *testing.T) {
	t.Run("GETs /api/<surnameId>/followers", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetSurnameFollowers(context.Background(), "surname-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1/followers"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetSurnameFollowers(context.Background(), "surname-1", 2)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	})

	t.Run("decodes profile results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"profile-1"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetSurnameFollowers(context.Background(), "surname-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestGetSurnameProfiles_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.GetSurnameProfiles(context.Background(), "surname-1", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1/profiles"))
}
