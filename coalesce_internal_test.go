package geni

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni/transport"
)

// Direct unit tests for transport.BulkCoalescer's two methods. The
// coalesce_test.go file in this package proves the *integrated*
// concurrency behaviour by spawning real goroutines through
// doRequest; these tests poke PrepareBulkRequest and
// ParseBulkResponse directly with a synthetic urlMap so the
// coalescing logic is verifiable without goroutine timing.

// All tests below run against "profile-1" as the calling request's
// id — the actual id is irrelevant to what's being verified (the
// urlMap routing logic). Hardcoding it keeps the helper signature
// minimal and the test bodies free of magic strings.
const coalescerTestCurrentId = "profile-1"

// newCoalescer reproduces the same instantiation GetProfile makes.
// Keeping it private and inline mirrors how each Get* singular sets
// up its own coalescer; if those ever diverge, the per-resource
// tests in bulk_test.go will catch it.
func newCoalescer() *transport.BulkCoalescer[ProfileResponse, ProfileBulkResponse] {
	return &transport.BulkCoalescer[ProfileResponse, ProfileBulkResponse]{
		CurrentID: coalescerTestCurrentId,
		IDPrefix:  "profile",
		DecodeBulk: func(body []byte) (ProfileBulkResponse, error) {
			var e ProfileBulkResponse
			err := json.Unmarshal(body, &e)
			return e, err
		},
		ListResults: func(env ProfileBulkResponse) []ProfileResponse { return env.Results },
		IDOfResult:  func(p ProfileResponse) string { return p.Id },
	}
}

func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	t.Run("currentId is never duplicated in ids=", func(t *testing.T) {
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
		// Count occurrences of "profile-1" — should appear exactly
		// once, even though urlMap has it under its own key.
		count := 0
		for _, id := range splitCSV(gotIds) {
			if id == "profile-1" {
				count++
			}
		}
		Expect(count).To(Equal(1), "currentId appeared %d times in ids=, want 1; got %q", count, gotIds)
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

		// Singleton → no ids= param.
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

		gotIds := req.URL.Query().Get("ids")
		ids := splitCSV(gotIds)
		Expect(ids).To(ConsistOf("profile-1", "profile-3"),
			"profile-2 already had a cached response; coalescer should have excluded it")
	})
}

func TestCoalescer_ParseBulkResponse(t *testing.T) {
	t.Run("URL has no ids= → body returned unchanged", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1")
		body := []byte(`{"id":"profile-1","first_name":"A"}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(Equal(body))
	})

	t.Run("bulk body fanout: own id returned, siblings stored in urlMap, their cancels triggered", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}

		// Track the sibling cancellations so the test can assert
		// they fired.
		sib2Ctx, sib2Cancel := context.WithCancel(context.Background())
		sib3Ctx, sib3Cancel := context.WithCancel(context.Background())
		urlMap.Store("profile-2", sib2Cancel)
		urlMap.Store("profile-3", sib3Cancel)

		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2,profile-3")
		body := []byte(`{"results":[
			{"id":"profile-1","first_name":"A"},
			{"id":"profile-2","first_name":"B"},
			{"id":"profile-3","first_name":"C"}
		]}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())

		// Own id: returned as bytes that decode into the right
		// profile.
		var ownProfile ProfileResponse
		Expect(json.Unmarshal(out, &ownProfile)).To(Succeed())
		Expect(ownProfile.Id).To(Equal("profile-1"))

		// Siblings: urlMap entries swapped from CancelFunc to
		// []byte, and the prior CancelFuncs were called.
		Expect(sib2Ctx.Err()).To(Equal(context.Canceled))
		Expect(sib3Ctx.Err()).To(Equal(context.Canceled))

		for _, sibId := range []string{"profile-2", "profile-3"} {
			v, ok := urlMap.Load(sibId)
			Expect(ok).To(BeTrue(), "expected %s in urlMap", sibId)
			raw, ok := v.([]byte)
			Expect(ok).To(BeTrue(), "expected %s to be []byte, got %T", sibId, v)
			var got ProfileResponse
			Expect(json.Unmarshal(raw, &got)).To(Succeed())
			Expect(got.Id).To(Equal(sibId))
		}
	})

	t.Run("bulk body without our own id returns nil bytes", func(t *testing.T) {
		// Edge case: Geni omits our requested id from the bulk
		// response (e.g. it's been merged away). The coalescer
		// fans out whatever IS there; our own slot is nil so
		// the caller gets nil bytes back — doRequest's existing
		// behaviour ends up unmarshaling nil into a zero-value
		// struct. The test pins what the coalescer does, not
		// what the caller does with it.
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2")
		body := []byte(`{"results":[{"id":"profile-2","first_name":"B"}]}`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(BeNil())
	})

	t.Run("malformed bulk body returns the decode error", func(t *testing.T) {
		RegisterTestingT(t)
		urlMap := &sync.Map{}
		req := mustRequest(t, "https://example.com/api/profile-1?ids=profile-1,profile-2")
		body := []byte(`not json`)

		out, err := newCoalescer().ParseBulkResponse(req, body, urlMap)

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
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
