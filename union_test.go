package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

func TestGetUnion1(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)

	unionId := "union-1838"

	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	union, err := client.GetUnion(t.Context(), unionId)

	Expect(err).ToNot(HaveOccurred())
	Expect(union.Id).To(BeEquivalentTo(unionId))
}

func TestAddPartnerToUnion_Request(t *testing.T) {
	t.Run("POSTs to /api/<unionId>/add-partner", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-200"}`)

		_, err := c.AddPartnerToUnion(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9/add-partner"))
	})

	t.Run("decodes the new partner profile", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"id":"profile-200","first_name":"NewPartner","is_alive":true,"public":true}`
		c, _ := newFakeClient(http.StatusOK, body)

		partner, err := c.AddPartnerToUnion(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(partner.Id).To(Equal("profile-200"))
		Expect(partner.FirstName).ToNot(BeNil())
		Expect(*partner.FirstName).To(Equal("NewPartner"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.AddPartnerToUnion(context.Background(), "union-9")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.AddPartnerToUnion(context.Background(), "union-9")

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestAddChildToUnion_Request(t *testing.T) {
	t.Run("POSTs to /api/<unionId>/add-child without modifier by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChildToUnion(context.Background(), "union-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union-9/add-child"))
		Expect(ft.lastRequest.URL.Query().Has("relationship_modifier")).To(BeFalse())
	})

	t.Run("WithModifier sets the relationship_modifier query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-201"}`)

		_, err := c.AddChildToUnion(context.Background(), "union-9", WithModifier("adopt"))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
	})

	t.Run("decodes the new child profile", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"id":"profile-201","first_name":"NewChild","is_alive":true,"public":true}`
		c, _ := newFakeClient(http.StatusOK, body)

		child, err := c.AddChildToUnion(context.Background(), "union-9", WithModifier("foster"))

		Expect(err).ToNot(HaveOccurred())
		Expect(child.Id).To(Equal("profile-201"))
		Expect(child.FirstName).ToNot(BeNil())
		Expect(*child.FirstName).To(Equal("NewChild"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.AddChildToUnion(context.Background(), "union-9")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}
