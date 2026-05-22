package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

// Direct unit tests for BulkCoalescer's two methods. The root-package
// coalesce_test.go proves the *integrated* concurrency behaviour by
// spawning real goroutines through Client.Do; these tests poke
// PrepareBulkRequest and ParseBulkResponse directly with a synthetic
// urlMap so the coalescing logic is verifiable without goroutine
// timing.
//
// coalItem / coalEnvelope are minimal stand-ins for a resource type
// and its bulk envelope — the coalescer only needs an id accessor and
// a results slice, so the test stays self-contained rather than
// importing a real resource package.

type coalItem struct {
	ID string `json:"id"`
}

type coalEnvelope struct {
	Results []coalItem `json:"results"`
}

// coalescerTestCurrentID is the calling request's id for every test
// below — the actual value is irrelevant to what's verified (the
// urlMap routing logic); hardcoding keeps the test bodies free of
// magic strings.
const coalescerTestCurrentID = "profile-1"

func newCoalescer() *BulkCoalescer[coalItem, coalEnvelope] {
	return &BulkCoalescer[coalItem, coalEnvelope]{
		CurrentID: coalescerTestCurrentID,
		IDPrefix:  "profile",
		DecodeBulk: func(body []byte) (coalEnvelope, error) {
			var e coalEnvelope
			err := json.Unmarshal(body, &e)
			return e, err
		},
		ListResults: func(env coalEnvelope) []coalItem { return env.Results },
		IDOfResult:  func(p coalItem) string { return p.ID },
	}
}

func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	return req
}

// dummyCancel returns a context.CancelFunc that does nothing — used
// as a stand-in for the "still queued, waiting on limiter" sentinel
// the coalescer looks for in urlMap.
func dummyCancel() context.CancelFunc {
	_, cancel := context.WithCancel(context.Background())
	return cancel
}

func TestCoalescer_PrepareBulkRequest(t *testing.T) {
	t.Run("no siblings → URL unchanged (no ids= param)", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1")
		urlMap.Store("profile-1", dummyCancel())

		newCoalescer().PrepareBulkRequest(req, urlMap)

		Expect(req.URL.Query().Has("ids")).To(BeFalse())
	})

	t.Run("two siblings → ids= contains all three", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		urlMap.Store("profile-1", dummyCancel())
		urlMap.Store("profile-2", dummyCancel())
		urlMap.Store("profile-3", dummyCancel())
		req := mustRequest(t, "https://example.com/api/profile-1")

		newCoalescer().PrepareBulkRequest(req, urlMap)

		Expect(req.URL.Query().Has("ids")).To(BeTrue())
		gotIds := req.URL.Query().Get("ids")
		Expect(gotIds).To(ContainSubstring("profile-1"))
		Expect(gotIds).To(ContainSubstring("profile-2"))
		Expect(gotIds).To(ContainSubstring("profile-3"))
	})

	t.Run("currentID is never duplicated in ids=", func(t *testing.T) {
		// Regression on the explicit duplicate-id fix: the
		// current request's own urlMap entry must not get
		// appended a second time.
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		urlMap.Store("profile-1", dummyCancel())
		urlMap.Store("profile-2", dummyCancel())
		req := mustRequest(t, "https://example.com/api/profile-1")

		newCoalescer().PrepareBulkRequest(req, urlMap)

		gotIds := req.URL.Query().Get("ids")
		count := 0
		for _, id := range splitCSV(gotIds) {
			if id == "profile-1" {
				count++
			}
		}
		Expect(count).To(Equal(1), "currentID appeared %d times in ids=, want 1; got %q", count, gotIds)
	})

	t.Run("other resource families are not pulled in", func(t *testing.T) {
		// Verifies the IDPrefix filter excludes mismatched
		// resource families even when they're concurrently
		// queued in the same urlMap.
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		urlMap.Store("profile-1", dummyCancel())
		urlMap.Store("union-9", dummyCancel())
		urlMap.Store("document-7", dummyCancel())
		req := mustRequest(t, "https://example.com/api/profile-1")

		newCoalescer().PrepareBulkRequest(req, urlMap)

		Expect(req.URL.Query().Has("ids")).To(BeFalse())
	})

	t.Run("urlMap entries with non-CancelFunc values are skipped (already fanned-out)", func(t *testing.T) {
		// When a sibling request's response has already been
		// stored as []byte, it must not be re-fetched.
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		urlMap.Store("profile-1", dummyCancel())
		urlMap.Store("profile-2", []byte(`{"id":"profile-2"}`)) // already fanned-out
		urlMap.Store("profile-3", dummyCancel())
		req := mustRequest(t, "https://example.com/api/profile-1")

		newCoalescer().PrepareBulkRequest(req, urlMap)

		ids := splitCSV(req.URL.Query().Get("ids"))
		Expect(ids).To(ConsistOf("profile-1", "profile-3"),
			"profile-2 already had a cached response; coalescer should have excluded it")
	})
}

func TestCoalescer_ParseBulkResponse(t *testing.T) {
	t.Run("URL has no ids= → body returned unchanged", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1")
		body := []byte(`{"id":"profile-1"}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(Equal(body))
	})

	t.Run("bulk body fanout: own id returned, siblings stored, their cancels triggered", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}

		sib2Ctx, sib2Cancel := context.WithCancel(context.Background())
		sib3Ctx, sib3Cancel := context.WithCancel(context.Background())
		urlMap.Store("profile-2", sib2Cancel)
		urlMap.Store("profile-3", sib3Cancel)

		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2,profile-3")
		body := []byte(`{"results":[
			{"id":"profile-1"},{"id":"profile-2"},{"id":"profile-3"}
		]}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())

		var own coalItem
		Expect(json.Unmarshal(out, &own)).To(Succeed())
		Expect(own.ID).To(Equal("profile-1"))

		Expect(sib2Ctx.Err()).To(Equal(context.Canceled))
		Expect(sib3Ctx.Err()).To(Equal(context.Canceled))

		for _, sibID := range []string{"profile-2", "profile-3"} {
			v, ok := urlMap.Load(sibID)
			Expect(ok).To(BeTrue(), "expected %s in urlMap", sibID)
			raw, ok := v.([]byte)
			Expect(ok).To(BeTrue(), "expected %s to be []byte, got %T", sibID, v)
			var got coalItem
			Expect(json.Unmarshal(raw, &got)).To(Succeed())
			Expect(got.ID).To(Equal(sibID))
		}
	})

	t.Run("bulk body without our own id returns nil bytes", func(t *testing.T) {
		// Edge case: Geni omits our requested id from the bulk
		// response (e.g. it's been merged away). The coalescer
		// fans out whatever IS there; our own slot is nil so the
		// caller gets nil bytes back.
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2")
		body := []byte(`{"results":[{"id":"profile-2"}]}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(BeNil())
	})

	t.Run("malformed bulk body returns the decode error", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2")

		out, err := newCoalescer().ParseBulkResponse(req, []byte(`not json`), urlMap)

		Expect(err).To(HaveOccurred())
		Expect(out).To(BeNil())
	})
}

// splitCSV breaks "a,b,c" into ["a","b","c"]. Used by the
// prepare-bulk tests to verify ids= contents without depending on
// ordering (sync.Map.Range is undefined-order).
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := range len(s) {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
