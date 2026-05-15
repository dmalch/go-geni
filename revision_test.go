package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetRevision_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"revision-101","action":"create","timestamp":"2026-05-15T09:00:00Z"}`)

	r, err := c.GetRevision(context.Background(), "revision-101")

	Expect(err).ToNot(HaveOccurred())
	Expect(r.Id).To(Equal("revision-101"))
	Expect(r.Action).To(Equal("create"))
	Expect(r.Timestamp).To(Equal("2026-05-15T09:00:00Z"))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision-101"))
}

func TestGetRevisions_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"revision-101","action":"create"}`)

		res, err := c.GetRevisions(context.Background(), []string{"revision-101"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision-101"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("revision-101"))
	})

	t.Run("2 ids → /api/revision?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"revision-101"},{"id":"revision-102"}]}`)

		_, err := c.GetRevisions(context.Background(), []string{"revision-101", "revision-102"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("revision-101,revision-102"))
	})
}
