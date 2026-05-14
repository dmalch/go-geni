package geni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// --- paginate engine -------------------------------------------------------

func TestPaginate_YieldsAllItemsAcrossPages(t *testing.T) {
	RegisterTestingT(t)

	pages := [][]int{
		{1, 2, 3},
		{4, 5},
		{6},
	}
	fetch := func(_ context.Context, page int) ([]int, bool, error) {
		if page < 1 || page > len(pages) {
			return nil, false, fmt.Errorf("unexpected page %d", page)
		}
		return pages[page-1], page < len(pages), nil
	}

	var got []int
	for v, err := range paginate(context.Background(), fetch) {
		Expect(err).ToNot(HaveOccurred())
		got = append(got, *v)
	}

	Expect(got).To(Equal([]int{1, 2, 3, 4, 5, 6}))
}

func TestPaginate_StopsAtFirstError(t *testing.T) {
	RegisterTestingT(t)

	boom := errors.New("boom")
	fetch := func(_ context.Context, page int) ([]int, bool, error) {
		if page == 1 {
			return []int{1, 2}, true, nil
		}
		return nil, false, boom
	}

	var seen []int
	var gotErr error
	for v, err := range paginate(context.Background(), fetch) {
		if err != nil {
			gotErr = err
			Expect(v).To(BeNil())
			break
		}
		seen = append(seen, *v)
	}

	Expect(seen).To(Equal([]int{1, 2}))
	Expect(gotErr).To(MatchError(boom))
}

func TestPaginate_HonorsEarlyBreak(t *testing.T) {
	RegisterTestingT(t)

	called := 0
	fetch := func(_ context.Context, page int) ([]int, bool, error) {
		called++
		// Return many items; we should never request page 2 because
		// the caller breaks after the first item.
		return []int{page * 10, page*10 + 1, page*10 + 2}, true, nil
	}

	var got []int
	for v, err := range paginate(context.Background(), fetch) {
		Expect(err).ToNot(HaveOccurred())
		got = append(got, *v)
		break
	}

	Expect(got).To(Equal([]int{10}))
	Expect(called).To(Equal(1), "fetch should only have been called for page 1")
}

func TestPaginate_StopsWhenHasNextFalse(t *testing.T) {
	RegisterTestingT(t)

	called := 0
	fetch := func(_ context.Context, page int) ([]int, bool, error) {
		called++
		return []int{page}, false, nil
	}

	var got []int
	for v, err := range paginate(context.Background(), fetch) {
		Expect(err).ToNot(HaveOccurred())
		got = append(got, *v)
	}

	Expect(got).To(Equal([]int{1}))
	Expect(called).To(Equal(1))
}

// --- Iter* wrappers (smoke tests against the fake transport) ---------------

// pagedTransport returns a configurable sequence of paginated JSON
// bodies, one per HTTP request. When the body slice is exhausted it
// returns an error so a runaway iteration is obvious.
type pagedTransport struct {
	bodies []string
	calls  int
}

func (t *pagedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.calls >= len(t.bodies) {
		return nil, fmt.Errorf("pagedTransport: request %d exceeds configured bodies (%d)", t.calls+1, len(t.bodies))
	}
	body := t.bodies[t.calls]
	t.calls++
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func newPagedClient(bodies ...string) (*Client, *pagedTransport) {
	pt := &pagedTransport{bodies: bodies}
	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	c.client = &http.Client{Transport: pt}
	return c, pt
}

func TestIterManagedProfiles_WalksAllPages(t *testing.T) {
	RegisterTestingT(t)
	c, pt := newPagedClient(
		`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…"}`,
		`{"results":[{"id":"profile-3"}],"page":2}`,
	)

	var got []string
	for p, err := range c.IterManagedProfiles(context.Background()) {
		Expect(err).ToNot(HaveOccurred())
		got = append(got, p.Id)
	}

	Expect(got).To(Equal([]string{"profile-1", "profile-2", "profile-3"}))
	Expect(pt.calls).To(Equal(2))
}

func TestIterDocumentComments_WalksAllPages(t *testing.T) {
	RegisterTestingT(t)
	c, pt := newPagedClient(
		`{"results":[{"id":"c-1","comment":"a"},{"id":"c-2","comment":"b"}],"page":1,"next_page":"…"}`,
		`{"results":[{"id":"c-3","comment":"c"}],"page":2}`,
	)

	var ids []string
	for cm, err := range c.IterDocumentComments(context.Background(), "doc-1") {
		Expect(err).ToNot(HaveOccurred())
		ids = append(ids, cm.Id)
	}

	Expect(ids).To(Equal([]string{"c-1", "c-2", "c-3"}))
	Expect(pt.calls).To(Equal(2))
}

func TestIterProjectFollowers_WalksAllPages(t *testing.T) {
	RegisterTestingT(t)
	c, pt := newPagedClient(
		`{"results":[{"id":"profile-10"}],"page":1,"next_page":"…"}`,
		`{"results":[{"id":"profile-11"}],"page":2}`,
	)

	var ids []string
	for p, err := range c.IterProjectFollowers(context.Background(), "project-7") {
		Expect(err).ToNot(HaveOccurred())
		ids = append(ids, p.Id)
	}

	Expect(ids).To(Equal([]string{"profile-10", "profile-11"}))
	Expect(pt.calls).To(Equal(2))
}
