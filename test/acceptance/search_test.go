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

	// Searches against the sandbox may be eventually consistent —
	// a profile created milliseconds before the search call is not
	// guaranteed to appear. We assert call shape and pagination
	// envelope rather than membership of the freshly-created
	// fixture in the result set.
	It("returns a paginated response envelope for a name search", func() {
		// Use a high-entropy last name so any noise in the sandbox
		// doesn't contaminate the response.
		uniq := fmt.Sprintf("AccSearch%d", time.Now().UnixNano())
		createFixtureProfile(ctx, client, uniq)

		res, err := client.SearchProfiles(ctx, uniq, 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res).ToNot(BeNil())
		// `page` may be omitted when there's nothing to paginate;
		// either zero or 1 is acceptable for the first page.
		Expect(res.Page).To(BeNumerically("<=", 1))
	})

})
