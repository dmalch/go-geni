package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetUser_Request(t *testing.T) {
	t.Run("GETs /api/user", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"user-42","name":"Test","account_type":"basic"}`)

		user, err := c.GetUser(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(user.Id).To(Equal("user-42"))
		Expect(user.Name).To(Equal("Test"))
		Expect(user.AccountType).To(Equal("basic"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user"))
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.GetUser(context.Background())

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}
