package geni

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

// countingTransport observes how many times the client actually hit
// the network and what URL each call carried. Used to verify
// bulk-coalescing collapses N concurrent singular reads into one
// (or a small number of) bulk requests.
type countingTransport struct {
	mu          sync.Mutex
	requests    []*http.Request
	bodyBuilder func(req *http.Request) (status int, body []byte)
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.requests = append(t.requests, req.Clone(req.Context()))
	t.mu.Unlock()

	status := http.StatusOK
	var body []byte
	if t.bodyBuilder != nil {
		status, body = t.bodyBuilder(req)
	}
	if body == nil {
		body = []byte("{}")
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func (t *countingTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.requests)
}

func (t *countingTransport) snapshot() []*http.Request {
	t.mu.Lock()
	defer t.mu.Unlock()
	clone := make([]*http.Request, len(t.requests))
	copy(clone, t.requests)
	return clone
}

// newClientWithCountingTransport builds a sandbox-mode Client whose
// limiter is slow enough that concurrent calls queue up while the
// test runs — giving coalescing a chance to sweep the urlMap.
func newClientWithCountingTransport(builder func(req *http.Request) (status int, body []byte)) (*Client, *countingTransport) {
	ct := &countingTransport{bodyBuilder: builder}
	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	c.client = &http.Client{Transport: ct}
	// 1 token / 500ms, burst 1 — first goroutine consumes burst
	// immediately, the rest queue and get coalesced when the
	// first sweeps urlMap.
	c.limiter = rate.NewLimiter(rate.Every(500*time.Millisecond), 1)
	// Pre-consume the burst so even the first goroutine has to
	// wait — guarantees all N concurrent calls queue together.
	_ = c.limiter.Wait(context.Background())
	return c, ct
}

// idsFromQuery parses a "?ids=A,B,C" param into a slice. Returns nil
// for requests that didn't pass ids.
func idsFromQuery(u *url.URL) []string {
	raw := u.Query().Get("ids")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// runCoalesceTest covers the common shape of the per-resource
// coalescing tests: spawn N concurrent goroutines, verify the
// transport saw a single bulk request carrying all N ids.
func runCoalesceTest(
	t *testing.T,
	idPrefix string,
	fetch func(c *Client, ctx context.Context, id string) error,
) {
	t.Helper()
	RegisterTestingT(t)

	const numCalls = 4
	ids := make([]string, numCalls)
	idSet := map[string]bool{}
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%d", idPrefix, 100+i)
		idSet[ids[i]] = true
	}

	c, ct := newClientWithCountingTransport(func(req *http.Request) (int, []byte) {
		// Respond with a bulk envelope that contains each
		// requested id verbatim. Only-ids responses are
		// deliberately not used here since the coalescer adds
		// ?ids= for >1 request and we want that path exercised.
		reqIds := idsFromQuery(req.URL)
		results := make([]string, 0, len(reqIds))
		for _, id := range reqIds {
			results = append(results, fmt.Sprintf(`{"id":%q}`, id))
		}
		body := fmt.Sprintf(`{"results":[%s]}`, strings.Join(results, ","))
		return http.StatusOK, []byte(body)
	})

	var wg sync.WaitGroup
	var errs atomic.Int32
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := fetch(c, context.Background(), id); err != nil {
				errs.Add(1)
				t.Logf("fetch(%s) failed: %v", id, err)
			}
		}(id)
	}
	wg.Wait()

	Expect(errs.Load()).To(BeEquivalentTo(0), "expected all fetches to succeed")

	// At minimum we expect *strictly fewer* HTTP requests than
	// goroutines — proves coalescing happened. The exact count is
	// timing-dependent; in practice with the slow limiter it
	// collapses to 1.
	Expect(ct.count()).To(BeNumerically("<", numCalls),
		"coalescing should collapse %d concurrent reads into fewer requests; saw %d",
		numCalls, ct.count())

	// Every requested id must appear in the union of ?ids= params
	// across all observed HTTP requests.
	gotIds := map[string]bool{}
	for _, r := range ct.snapshot() {
		if r.URL.Query().Has("ids") {
			for _, id := range idsFromQuery(r.URL) {
				gotIds[id] = true
			}
		} else {
			// singular fallback: id is in the path
			last := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			gotIds[last] = true
		}
	}
	for id := range idSet {
		Expect(gotIds[id]).To(BeTrue(), "id %s was not requested", id)
	}
}

func TestCoalesce_GetProfile(t *testing.T) {
	runCoalesceTest(t, "profile", func(c *Client, ctx context.Context, id string) error {
		_, err := c.GetProfile(ctx, id)
		return err
	})
}

func TestCoalesce_GetUnion(t *testing.T) {
	runCoalesceTest(t, "union", func(c *Client, ctx context.Context, id string) error {
		_, err := c.GetUnion(ctx, id)
		return err
	})
}

func TestCoalesce_GetDocument(t *testing.T) {
	runCoalesceTest(t, "document", func(c *Client, ctx context.Context, id string) error {
		_, err := c.GetDocument(ctx, id)
		return err
	})
}

func TestCoalesce_GetPhoto(t *testing.T) {
	runCoalesceTest(t, "photo", func(c *Client, ctx context.Context, id string) error {
		_, err := c.GetPhoto(ctx, id)
		return err
	})
}

func TestCoalesce_GetVideo(t *testing.T) {
	runCoalesceTest(t, "video", func(c *Client, ctx context.Context, id string) error {
		_, err := c.GetVideo(ctx, id)
		return err
	})
}

// TestCoalesce_DifferentResourcesDoNotMerge verifies that a profile
// request and a union request issued concurrently do NOT coalesce
// into one bulk call. The idPrefix filter on bulkCoalescer should
// keep them on separate URL dispatch paths.
func TestCoalesce_DifferentResourcesDoNotMerge(t *testing.T) {
	RegisterTestingT(t)

	c, ct := newClientWithCountingTransport(func(req *http.Request) (int, []byte) {
		// Empty bulk envelope is fine for this check — we're
		// asserting on URL paths, not on response decoding.
		return http.StatusOK, []byte(`{"results":[]}`)
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = c.GetProfile(context.Background(), "profile-1")
	}()
	go func() {
		defer wg.Done()
		_, _ = c.GetUnion(context.Background(), "union-1")
	}()
	wg.Wait()

	// Two distinct resource families → two distinct URL paths,
	// even though both went through the coalescer.
	requests := ct.snapshot()
	Expect(len(requests)).To(Equal(2))
	paths := []string{requests[0].URL.Path, requests[1].URL.Path}
	Expect(paths).To(ConsistOf(
		HaveSuffix("/api/profile-1"),
		HaveSuffix("/api/union-1"),
	))
}
