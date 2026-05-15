package geni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// bulkCoalescer wires the three internal doRequest options
// (withRequestKey / withPrepareBulkRequest / withParseBulkResponse)
// for one read of a single-fetchable resource. When two or more
// reads of the same resource type are in flight at the same time,
// the first to win the rate limiter sweeps the urlMap, appends the
// sibling ids to its outgoing request as `?ids=…`, parses the bulk
// response Geni returns, and fans the individual entries back into
// the urlMap so the sibling goroutines wake up with cached bodies
// instead of issuing duplicate HTTP requests.
//
// Used by GetProfile, GetUnion, GetDocument, GetPhoto, GetVideo.
// The four singular Get* methods all share this structure; the
// generic isolates the per-resource differences (id prefix used to
// filter urlMap keys, bulk envelope type, accessor functions).
type bulkCoalescer[Item any, Envelope any] struct {
	// currentId is the id of the resource the calling Get* method
	// is fetching — used both as the urlMap key and as the
	// "claim mine, fan out the rest" anchor in the bulk response.
	currentId string
	// idPrefix is the resource family prefix (e.g. "profile",
	// "union", "document"). urlMap keys with this prefix are
	// candidates for coalescing into the same bulk call.
	idPrefix string
	// decodeBulk decodes the raw response body into the resource's
	// bulk envelope.
	decodeBulk func([]byte) (Envelope, error)
	// listResults returns the slice of items inside the envelope.
	listResults func(Envelope) []Item
	// idOfResult extracts the id of a single item — used to route
	// each fanned-out result back to the right urlMap key.
	idOfResult func(Item) string
}

func (b *bulkCoalescer[Item, Envelope]) options() []func(*opt) {
	return []func(*opt){
		withRequestKey(b.requestKey),
		withPrepareBulkRequest(b.prepareBulkRequest),
		withParseBulkResponse(b.parseBulkResponse),
	}
}

func (b *bulkCoalescer[Item, Envelope]) requestKey() string {
	return b.currentId
}

func (b *bulkCoalescer[Item, Envelope]) prepareBulkRequest(req *http.Request, urlMap *sync.Map) {
	ids := []string{b.currentId}
	prefix := b.idPrefix + "-"

	urlMap.Range(func(key, value any) bool {
		// Only entries that are still cancel funcs are
		// candidates — entries whose value is a []byte have
		// already been fanned-out responses from another bulk
		// pass and shouldn't be re-fetched.
		if _, ok := value.(context.CancelFunc); !ok {
			return true
		}
		keyString, ok := key.(string)
		if !ok || !strings.HasPrefix(keyString, prefix) {
			return true
		}
		// Skip the calling request's own entry — already in the
		// list as currentId.
		if keyString == b.currentId {
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

func (b *bulkCoalescer[Item, Envelope]) parseBulkResponse(req *http.Request, body []byte, urlMap *sync.Map) ([]byte, error) {
	// Singular path: the URL stayed as /api/<id> with no `ids=`
	// query param — the response is a single Item, not a bulk
	// envelope. Hand it back unchanged.
	if !req.URL.Query().Has("ids") {
		return body, nil
	}

	envelope, err := b.decodeBulk(body)
	if err != nil {
		slog.Error("Error unmarshaling bulk response", "error", err)
		return nil, err
	}

	var requestedRes []byte
	for _, item := range b.listResults(envelope) {
		jsonBody, err := json.Marshal(item)
		if err != nil {
			slog.Error("Error marshaling item", "error", err)
			return nil, err
		}
		id := b.idOfResult(item)
		if id == b.currentId {
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
