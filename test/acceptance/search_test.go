package acceptance

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// Geni's /profile/search requires `names` — calling without it returns
// a 500 ApiException ("You must specify a name or family member's name
// in your search."). The wire-shape "client omits empty names" is
// covered by the in-process unit suite; we don't re-probe that path
// against the live sandbox.

var _ = Describe("SearchProfiles", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	// Skipped: search indexing of freshly-created profiles is too
	// slow to assert against in CI — polled 60s with a high-entropy
	// unique name and the profile never appeared. The intended
	// Eventually membership assertion is preserved so unskipping is
	// a one-line change once the sandbox index speeds up (or once
	// we want a long-running scheduled job).
	It("eventually returns the freshly-created profile in search results", func() {
		Skip("sandbox search index doesn't reflect freshly-created profiles within 60s")

		uniq := fmt.Sprintf("AccSearch%d", time.Now().UnixNano())
		created := createFixtureProfile(ctx, client, uniq)

		Eventually(func(g Gomega) {
			res, err := client.SearchProfiles(ctx, uniq, 1)
			g.Expect(err).ToNot(HaveOccurred())
			ids := make([]string, 0, len(res.Results))
			for _, p := range res.Results {
				ids = append(ids, p.Id)
			}
			g.Expect(ids).To(ContainElement(created.Id))
		}).
			WithTimeout(60 * time.Second).
			WithPolling(3 * time.Second).
			Should(Succeed())
	})

})
