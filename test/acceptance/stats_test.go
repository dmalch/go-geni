package acceptance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

var _ = Describe("Stats API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("Get", func() {
		It("returns the platform's stats list", func() {
			res, err := client.Stats().Get(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			// The sandbox should always have at least one stat;
			// we don't assert specific names since they're opaque.
			Expect(res.Stats).ToNot(BeEmpty())
		})
	})
})
