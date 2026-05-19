package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
)

// Concurrent-read specs that exercise the bulk-read coalescing
// machinery against the live Geni sandbox. The integration tier
// (coalesce_test.go in the root package) proves the wire-level
// collapse with a fakeTransport; these specs prove the live API
// actually accepts the `/api/<id>?ids=A,B,C` pattern and returns a
// bulk envelope the client can decode back into correct singular
// results.
//
// They don't try to assert "exactly K HTTP requests" — that depends
// on Geni's rate-limit headers and goroutine timing, neither of
// which is observable from inside a sandbox spec. The correctness
// assertion (every goroutine gets back the right id) is what
// matters: if the coalescing logic mis-routed a result, the wrong
// goroutine would see another goroutine's data.

// tinyPng returns a 1×1 PNG byte stream — small enough to upload
// many of without burning sandbox quota.
func coalesceTinyPng() io.Reader {
	GinkgoHelper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	Expect(png.Encode(&buf, img)).To(Succeed())
	return &buf
}

// runConcurrent calls fetch on each id in parallel and returns the
// observed (id-fetched → id-returned) mapping plus any errors. A
// correct coalescer maps each requested id to its own response.
func runConcurrent(ids []string, fetch func(id string) (string, error)) (map[string]string, int) {
	GinkgoHelper()
	var (
		mu      sync.Mutex
		results = map[string]string{}
		errs    atomic.Int32
	)

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer GinkgoRecover()
			defer wg.Done()
			got, err := fetch(id)
			if err != nil {
				errs.Add(1)
				return
			}
			mu.Lock()
			results[id] = got
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	return results, int(errs.Load())
}

var _ = Describe("Bulk-read coalescing (live sandbox)", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("GetProfile concurrent reads", func() {
		It("three concurrent reads each return the right profile id", func() {
			tag := fmt.Sprintf("Coalesce%d", time.Now().UnixNano())
			a := createFixtureProfile(ctx, client, tag+"A")
			b := createFixtureProfile(ctx, client, tag+"B")
			c := createFixtureProfile(ctx, client, tag+"C")
			ids := []string{a.Id, b.Id, c.Id}

			results, errs := runConcurrent(ids, func(id string) (string, error) {
				p, err := client.GetProfile(ctx, id)
				if err != nil {
					return "", err
				}
				return p.Id, nil
			})

			Expect(errs).To(Equal(0))
			Expect(results).To(HaveLen(3))
			for _, id := range ids {
				Expect(results[id]).To(Equal(id),
					"GetProfile(%s) returned id %q — coalescing may have mis-routed responses",
					id, results[id])
			}
		})
	})

	Describe("GetUnion concurrent reads", func() {
		It("three concurrent reads each return the right union id", func() {
			_, _, unionA := createCoupleAndUnion(ctx, client)
			_, _, unionB := createCoupleAndUnion(ctx, client)
			_, _, unionC := createCoupleAndUnion(ctx, client)
			ids := []string{unionA, unionB, unionC}

			results, errs := runConcurrent(ids, func(id string) (string, error) {
				u, err := client.GetUnion(ctx, id)
				if err != nil {
					return "", err
				}
				return u.Id, nil
			})

			Expect(errs).To(Equal(0))
			Expect(results).To(HaveLen(3))
			for _, id := range ids {
				Expect(results[id]).To(Equal(id),
					"GetUnion(%s) returned id %q — coalescing may have mis-routed responses",
					id, results[id])
			}
		})
	})

	Describe("GetDocument concurrent reads", func() {
		It("three concurrent reads each return the right document id", func() {
			tag := fmt.Sprintf("CoalesceDoc%d", time.Now().UnixNano())
			a := createFixtureDocument(ctx, client, tag+"A", "a")
			b := createFixtureDocument(ctx, client, tag+"B", "b")
			c := createFixtureDocument(ctx, client, tag+"C", "c")
			ids := []string{a.Id, b.Id, c.Id}

			results, errs := runConcurrent(ids, func(id string) (string, error) {
				d, err := client.Document().Get(ctx, id)
				if err != nil {
					return "", err
				}
				return d.Id, nil
			})

			Expect(errs).To(Equal(0))
			Expect(results).To(HaveLen(3))
			for _, id := range ids {
				Expect(results[id]).To(Equal(id),
					"GetDocument(%s) returned id %q — coalescing may have mis-routed responses",
					id, results[id])
			}
		})
	})

	Describe("GetPhoto concurrent reads", func() {
		It("three concurrent reads each return the right photo id", func() {
			tag := fmt.Sprintf("CoalescePhoto%d", time.Now().UnixNano())
			a, err := client.Photo().Create(ctx, tag+"A", "a.png", coalesceTinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), a.Id) })
			b, err := client.Photo().Create(ctx, tag+"B", "b.png", coalesceTinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), b.Id) })
			cPhoto, err := client.Photo().Create(ctx, tag+"C", "c.png", coalesceTinyPng())
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), cPhoto.Id) })
			ids := []string{a.Id, b.Id, cPhoto.Id}

			results, errs := runConcurrent(ids, func(id string) (string, error) {
				p, err := client.Photo().Get(ctx, id)
				if err != nil {
					return "", err
				}
				return p.Id, nil
			})

			Expect(errs).To(Equal(0))
			Expect(results).To(HaveLen(3))
			for _, id := range ids {
				Expect(results[id]).To(Equal(id),
					"GetPhoto(%s) returned id %q — coalescing may have mis-routed responses",
					id, results[id])
			}
		})
	})

	Describe("GetVideo concurrent reads", func() {
		// Skipped pre-emptively: video specs need a real encoded
		// video fixture (Geni runs uploads through ffmpeg
		// validation; placeholder byte payloads fail). The
		// coalescing wiring is identical to Photo's, which is
		// already covered above.
		It("three concurrent reads each return the right video id", func() {
			Skip("requires a real video fixture — see CreateVideo godoc; Photo's coalescing spec proves the pattern")
		})
	})
})
