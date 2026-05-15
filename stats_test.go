package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetStats_Request(t *testing.T) {
	t.Run("GETs /api/stats", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"stats":[{"name":"total_profiles","value":1234567}]}`)

		res, err := c.GetStats(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Stats).To(HaveLen(1))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/stats"))
	})

	t.Run("empty stats array decodes cleanly", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusOK, `{"stats":[]}`)

		res, err := c.GetStats(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Stats).To(BeEmpty())
	})
}
