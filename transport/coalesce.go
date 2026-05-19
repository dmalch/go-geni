package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// Coalescer is the contract that lets a single in-flight read of a
// resource piggy-back sibling reads of the same resource type into one
// bulk call. See BulkCoalescer for the generic implementation that
// the Profile/Union/Document/Photo/Video resources use; resources can
// also supply their own implementation if the wire shape diverges.
type Coalescer interface {
	// RequestKey returns the urlMap key identifying this request's
	// resource. The first call to win the limiter sweeps the urlMap
	// for siblings with matching prefixes.
	RequestKey() string

	// PrepareBulkRequest is called once this request has won the
	// limiter, before it is sent. The implementation may inspect
	// urlMap to discover sibling reads of the same resource and
	// extend req with an `ids=` query param.
	PrepareBulkRequest(req *http.Request, urlMap *sync.Map)

	// ParseBulkResponse is called after a successful HTTP response.
	// When the request was bulked (req has `ids=`), the implementation
	// fans the bulk envelope back into urlMap so sibling goroutines
	// pick up cached bodies, and returns the slice belonging to the
	// caller. When the request stayed singular, the implementation
	// returns body unchanged.
	ParseBulkResponse(req *http.Request, body []byte, urlMap *sync.Map) ([]byte, error)
}

// BulkCoalescer is the generic Coalescer used by every resource that
// has a "bulk-get by ids" endpoint. When two or more reads of the
// same resource type are in flight at the same time, the first to win
// the rate limiter sweeps the urlMap, appends the sibling ids to its
// outgoing request as `?ids=…`, parses the bulk response Geni returns,
// and fans the individual entries back into the urlMap so the sibling
// goroutines wake up with cached bodies instead of issuing duplicate
// HTTP requests.
//
// Used by Profile/Union/Document/Photo/Video. The four singular Get*
// methods all share this structure; the generic isolates the
// per-resource differences (id prefix used to filter urlMap keys, bulk
// envelope type, accessor functions).
type BulkCoalescer[Item any, Envelope any] struct {
	// CurrentID is the id of the resource the calling Get* method is
	// fetching — used both as the urlMap key and as the "claim mine,
	// fan out the rest" anchor in the bulk response.
	CurrentID string
	// IDPrefix is the resource family prefix (e.g. "profile",
	// "union", "document"). urlMap keys with this prefix are
	// candidates for coalescing into the same bulk call.
	IDPrefix string
	// DecodeBulk decodes the raw response body into the resource's
	// bulk envelope.
	DecodeBulk func([]byte) (Envelope, error)
	// ListResults returns the slice of items inside the envelope.
	ListResults func(Envelope) []Item
	// IDOfResult extracts the id of a single item — used to route
	// each fanned-out result back to the right urlMap key.
	IDOfResult func(Item) string
}

// RequestKey implements Coalescer.
func (b *BulkCoalescer[Item, Envelope]) RequestKey() string { return b.CurrentID }

// PrepareBulkRequest implements Coalescer.
func (b *BulkCoalescer[Item, Envelope]) PrepareBulkRequest(req *http.Request, urlMap *sync.Map) {
	ids := []string{b.CurrentID}
	prefix := b.IDPrefix + "-"

	urlMap.Range(func(key, value any) bool {
		// Only entries that are still cancel funcs are candidates —
		// entries whose value is a []byte have already been fanned-out
		// responses from another bulk pass and shouldn't be re-fetched.
		if _, ok := value.(context.CancelFunc); !ok {
			return true
		}
		keyString, ok := key.(string)
		if !ok || !strings.HasPrefix(keyString, prefix) {
			return true
		}
		// Skip the calling request's own entry — already in the list
		// as CurrentID.
		if keyString == b.CurrentID {
			return true
		}
		ids = append(ids, keyString)
		return true
	})

	if len(ids) > 1 {
		query := req.URL.Query()
		query.Add("ids", strings.Join(ids, ","))
		req.URL.RawQuery = query.Encode()
	}
}

// ParseBulkResponse implements Coalescer.
func (b *BulkCoalescer[Item, Envelope]) ParseBulkResponse(req *http.Request, body []byte, urlMap *sync.Map) ([]byte, error) {
	// Singular path: the URL stayed as /api/<id> with no `ids=` query
	// param — the response is a single Item, not a bulk envelope.
	// Hand it back unchanged.
	if !req.URL.Query().Has("ids") {
		return body, nil
	}

	envelope, err := b.DecodeBulk(body)
	if err != nil {
		slog.Error("Error unmarshaling bulk response", "error", err)
		return nil, err
	}

	var requestedRes []byte
	for _, item := range b.ListResults(envelope) {
		jsonBody, err := json.Marshal(item)
		if err != nil {
			slog.Error("Error marshaling item", "error", err)
			return nil, err
		}
		id := b.IDOfResult(item)
		if id == b.CurrentID {
			requestedRes = jsonBody
			continue
		}
		previous, loaded := urlMap.Swap(id, jsonBody)
		if loaded {
			if cancelFunc, ok := previous.(context.CancelFunc); ok {
				cancelFunc()
			}
		}
	}
	return requestedRes, nil
}
